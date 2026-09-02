package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/catalog"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
)

func catalogUseCase(t *testing.T, db *gorm.DB) *CatalogUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	weatherUseCase := NewWeatherUseCase(db, log, &repository.WeatherRepository{}, nil)
	projection := NewProjectionUseCase(db, log,
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.CalibrationRepository{}, weatherUseCase)

	return NewCatalogUseCase(db, log, nil,
		&repository.CooperativeRepository{}, &repository.CommodityRepository{},
		&repository.BlockRepository{}, &repository.VarietyRepository{}, projection)
}

func supplyRequestUseCase(t *testing.T, db *gorm.DB) *SupplyRequestUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	return NewSupplyRequestUseCase(db, log, validator.New(),
		&repository.SupplyRequestRepository{}, catalogUseCase(t, db))
}

func commerceFixture(t *testing.T) (*gorm.DB, *entity.AppUser) {
	t.Helper()

	db := dashboardFixture(t)
	if err := db.AutoMigrate(&entity.SupplyContractRequest{}); err != nil {
		t.Fatalf("migrating supply_contract_request: %v", err)
	}

	organisation := "PT Pangan Nusantara"
	return db, &entity.AppUser{
		ID:           "buyer-1",
		Role:         constants.RoleBuyer,
		FullName:     "Diana",
		Organisation: &organisation,
	}
}

func onlyListing(t *testing.T, db *gorm.DB) catalog.Listing {
	t.Helper()

	listings, err := catalogUseCase(t, db).
		LoadForCooperative(context.Background(), homeCoop, projectionNow)
	if err != nil {
		t.Fatalf("LoadForCooperative: %v", err)
	}
	if len(listings) == 0 {
		t.Fatal("the cooperative offers no listings, want at least one")
	}
	return listings[0]
}

func TestCatalogLoadForCooperativeDerivesListingsFromTheProjection(t *testing.T) {
	db, _ := commerceFixture(t)

	listing := onlyListing(t, db)

	if listing.CooperativeID != homeCoop {
		t.Errorf("CooperativeID = %q, want the home cooperative", listing.CooperativeID)
	}
	if listing.CooperativeName != "KUD Subang" {
		t.Errorf("CooperativeName = %q, want KUD Subang", listing.CooperativeName)
	}
	if listing.CommodityName != "Jagung" {
		t.Errorf("CommodityName = %q, want Jagung", listing.CommodityName)
	}
	if listing.VarietyName == nil || *listing.VarietyName != "Bisi-18" {
		t.Errorf("VarietyName = %v, want Bisi-18", listing.VarietyName)
	}
	if listing.Tonnes <= 0 {
		t.Errorf("Tonnes = %v, want a positive tonnage", listing.Tonnes)
	}
}

func createRequest(listingID string) *model.CreateSupplyRequestRequest {
	return &model.CreateSupplyRequestRequest{
		ListingID:          listingID,
		VolumeTonnes:       2.5,
		DeliveryPreference: constants.DeliverToBuyerWarehouse,
		Notes:              "Mohon dikabari sebelum pengiriman.",
	}
}

func TestCreateSupplyRequestTakesTheWindowFromTheListingNotTheForm(t *testing.T) {
	db, buyer := commerceFixture(t)
	listing := onlyListing(t, db)

	stored, err := supplyRequestUseCase(t, db).
		Create(context.Background(), buyer, createRequest(listing.ID), projectionNow)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if stored.CooperativeID != listing.CooperativeID ||
		stored.CommodityID != listing.CommodityID {
		t.Errorf("stored = %+v, want the listing's cooperative and commodity", stored)
	}
	if !stored.WindowStart.Equal(listing.WeekStart) || !stored.WindowEnd.Equal(listing.WeekEnd) {
		t.Errorf("window = %v..%v, want the listing's %v..%v",
			stored.WindowStart, stored.WindowEnd, listing.WeekStart, listing.WeekEnd)
	}
	if stored.VolumeKg != 2500 {
		t.Errorf("VolumeKg = %v, want 2500", stored.VolumeKg)
	}
	if stored.Status != constants.RequestPending {
		t.Errorf("Status = %q, want %q", stored.Status, constants.RequestPending)
	}
}

func TestCreateSupplyRequestCopiesTheBuyerIdentityFromTheSession(t *testing.T) {
	db, buyer := commerceFixture(t)
	listing := onlyListing(t, db)

	stored, err := supplyRequestUseCase(t, db).
		Create(context.Background(), buyer, createRequest(listing.ID), projectionNow)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if stored.BuyerID != buyer.ID || stored.BuyerName != "Diana" {
		t.Errorf("buyer = %q / %q, want the session's", stored.BuyerID, stored.BuyerName)
	}
	if stored.BuyerOrganisation == nil || *stored.BuyerOrganisation != "PT Pangan Nusantara" {
		t.Errorf("BuyerOrganisation = %v, want the session's", stored.BuyerOrganisation)
	}
}

func TestCreateSupplyRequestRecordsTheDeliveryPreferenceInTheNotes(t *testing.T) {
	db, buyer := commerceFixture(t)
	listing := onlyListing(t, db)

	stored, err := supplyRequestUseCase(t, db).
		Create(context.Background(), buyer, createRequest(listing.ID), projectionNow)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if stored.Notes == nil {
		t.Fatal("Notes = nil, want the delivery preference recorded")
	}
	if !strings.Contains(*stored.Notes, "Antar ke gudang pembeli") {
		t.Errorf("Notes = %q, want the preference spelled out", *stored.Notes)
	}
	if !strings.Contains(*stored.Notes, "Mohon dikabari") {
		t.Errorf("Notes = %q, want the buyer's own note kept", *stored.Notes)
	}
}

func TestCreateSupplyRequestRefusesAnUnusableListing(t *testing.T) {
	db, buyer := commerceFixture(t)
	useCase := supplyRequestUseCase(t, db)

	tests := []struct {
		name      string
		listingID string
		want      error
	}{
		{"malformed id", "not-a-listing", ErrListingUnknown},
		{"well formed but not offered",
			catalog.ListingID(homeCoop, "22222222-2222-4222-8222-222222222222", "2020-W01"),
			ErrListingGone},
	}

	for _, test := range tests {
		_, err := useCase.Create(
			context.Background(), buyer, createRequest(test.listingID), projectionNow)

		if !errors.Is(err, test.want) {
			t.Errorf("%s: err = %v, want %v", test.name, err, test.want)
		}
	}
}

func TestCreateSupplyRequestRejectsANonPositiveVolume(t *testing.T) {
	db, buyer := commerceFixture(t)
	listing := onlyListing(t, db)

	request := createRequest(listing.ID)
	request.VolumeTonnes = 0

	if _, err := supplyRequestUseCase(t, db).
		Create(context.Background(), buyer, request, projectionNow); err == nil {
		t.Error("Create returned nil error, want a rejection")
	}
}

func pengurusOf(cooperativeID string) *entity.AppUser {
	return &entity.AppUser{
		ID: "pengurus-1", Role: constants.RolePengurus, CooperativeID: &cooperativeID,
	}
}

func TestRespondToRequestAnswersOnlyItsOwnCooperativesRequests(t *testing.T) {
	db, buyer := commerceFixture(t)
	listing := onlyListing(t, db)
	useCase := supplyRequestUseCase(t, db)

	stored, err := useCase.Create(
		context.Background(), buyer, createRequest(listing.ID), projectionNow)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	stranger := pengurusOf(otherCoop)
	err = useCase.Respond(context.Background(), stranger, stored.ID,
		&model.RespondToRequestRequest{Decision: constants.RequestAccepted}, projectionNow)
	if !errors.Is(err, ErrRequestNotFound) {
		t.Errorf("err = %v, want ErrRequestNotFound for another cooperative's pengurus", err)
	}

	unchanged := new(entity.SupplyContractRequest)
	if err := db.Where("id = ?", stored.ID).Take(unchanged).Error; err != nil {
		t.Fatalf("reading back the request: %v", err)
	}
	if unchanged.Status != constants.RequestPending {
		t.Errorf("Status = %q, want it untouched at %q", unchanged.Status, constants.RequestPending)
	}
}

func TestRespondToRequestStampsTheDecisionAndTheTime(t *testing.T) {
	db, buyer := commerceFixture(t)
	listing := onlyListing(t, db)
	useCase := supplyRequestUseCase(t, db)

	stored, err := useCase.Create(
		context.Background(), buyer, createRequest(listing.ID), projectionNow)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := useCase.Respond(context.Background(), pengurusOf(homeCoop), stored.ID,
		&model.RespondToRequestRequest{Decision: constants.RequestDeclined},
		projectionNow); err != nil {
		t.Fatalf("Respond: %v", err)
	}

	answered := new(entity.SupplyContractRequest)
	if err := db.Where("id = ?", stored.ID).Take(answered).Error; err != nil {
		t.Fatalf("reading back the request: %v", err)
	}
	if answered.Status != constants.RequestDeclined {
		t.Errorf("Status = %q, want %q", answered.Status, constants.RequestDeclined)
	}
	if answered.RespondedAt == nil {
		t.Error("RespondedAt = nil, want the answer stamped")
	}
}

func TestRespondToRequestKeepsAcceptedTonnageWithinTheProjection(t *testing.T) {
	db, buyer := commerceFixture(t)
	listing := onlyListing(t, db)
	useCase := supplyRequestUseCase(t, db)
	pengurus := pengurusOf(homeCoop)

	first := createRequest(listing.ID)
	first.VolumeTonnes = listing.Tonnes
	storedFirst, err := useCase.Create(context.Background(), buyer, first, projectionNow)
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	if err := useCase.Respond(context.Background(), pengurus, storedFirst.ID,
		&model.RespondToRequestRequest{Decision: constants.RequestAccepted}, projectionNow); err != nil {
		t.Fatalf("accepting the whole projection should succeed: %v", err)
	}

	second := createRequest(listing.ID)
	second.VolumeTonnes = 0.01
	storedSecond, err := useCase.Create(context.Background(), buyer, second, projectionNow)
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	err = useCase.Respond(context.Background(), pengurus, storedSecond.ID,
		&model.RespondToRequestRequest{Decision: constants.RequestAccepted}, projectionNow)
	if !errors.Is(err, ErrAllocationExceeded) {
		t.Errorf("err = %v, want ErrAllocationExceeded once the projection is fully committed", err)
	}

	unchanged := new(entity.SupplyContractRequest)
	if err := db.Where("id = ?", storedSecond.ID).Take(unchanged).Error; err != nil {
		t.Fatalf("reading back the second request: %v", err)
	}
	if unchanged.Status != constants.RequestPending {
		t.Errorf("Status = %q, want it left pending after the cap refused it", unchanged.Status)
	}
}

func TestListSupplyRequestsSplitsByWhoIsAsking(t *testing.T) {
	db, buyer := commerceFixture(t)
	listing := onlyListing(t, db)
	useCase := supplyRequestUseCase(t, db)

	if _, err := useCase.Create(
		context.Background(), buyer, createRequest(listing.ID), projectionNow); err != nil {
		t.Fatalf("Create: %v", err)
	}

	forBuyer, err := useCase.List(context.Background(), buyer)
	if err != nil {
		t.Fatalf("List for buyer: %v", err)
	}
	if len(forBuyer) != 1 {
		t.Errorf("buyer sees %d requests, want 1", len(forBuyer))
	}

	forCooperative, err := useCase.List(context.Background(), pengurusOf(homeCoop))
	if err != nil {
		t.Fatalf("List for cooperative: %v", err)
	}
	if len(forCooperative) != 1 {
		t.Errorf("cooperative sees %d requests, want 1", len(forCooperative))
	}

	forStranger, err := useCase.List(context.Background(), pengurusOf(otherCoop))
	if err != nil {
		t.Fatalf("List for another cooperative: %v", err)
	}
	if len(forStranger) != 0 {
		t.Errorf("another cooperative sees %d requests, want 0", len(forStranger))
	}
}

func TestCatalogLoadCollectsFilterOptionsFromWhatIsListed(t *testing.T) {
	db, _ := commerceFixture(t)

	built, err := catalogUseCase(t, db).Load(context.Background(), projectionNow)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(built.Provinces) != 1 || built.Provinces[0] != "Jawa Barat" {
		t.Errorf("Provinces = %v, want only what is actually listed", built.Provinces)
	}
	for _, commodity := range built.Commodities {
		if commodity.Name == "" {
			t.Errorf("commodity %q has no name", commodity.ID)
		}
	}
	for _, listing := range built.Listings {
		if agronomy.DaysBetween(listing.WeekStart, listing.WeekEnd) != 6 {
			t.Errorf("listing %s spans %d days, want a full week",
				listing.ID, agronomy.DaysBetween(listing.WeekStart, listing.WeekEnd))
		}
	}
}
