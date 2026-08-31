package usecase

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
)

func rdkkUseCase(t *testing.T, db *gorm.DB) *RdkkUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	return NewRdkkUseCase(db, log,
		&repository.CooperativeRepository{}, &repository.PlotRepository{},
		&repository.BlockRepository{}, &repository.MemberRepository{},
		&repository.FertiliserRateRepository{}, &repository.InputOrderRepository{})
}

func rdkkFixture(t *testing.T) (*gorm.DB, *entity.AppUser) {
	t.Helper()

	db := dashboardFixture(t)
	if err := db.AutoMigrate(&entity.FertiliserRate{}); err != nil {
		t.Fatalf("migrating fertiliser_rate: %v", err)
	}

	rates := []entity.FertiliserRate{
		{CommodityID: "commodity-1", InputItem: "urea", KgPerHa: 250, Source: "Permentan 40/2007"},
		{CommodityID: "commodity-1", InputItem: "sp36", KgPerHa: 100, Source: "Permentan 40/2007"},
	}
	if err := db.Create(&rates).Error; err != nil {
		t.Fatalf("seeding fertiliser rates: %v", err)
	}

	cooperativeID := homeCoop
	return db, &entity.AppUser{
		ID: "pengurus-1", Role: constants.RolePengurus, CooperativeID: &cooperativeID,
	}
}

func TestRdkkLoadSeasonCoversOnlyTheSeasonsPlantings(t *testing.T) {
	db, user := rdkkFixture(t)

	document, err := rdkkUseCase(t, db).
		LoadSeason(context.Background(), *user.CooperativeID, DefaultSeason(projectionNow))
	if err != nil {
		t.Fatalf("LoadSeason: %v", err)
	}

	if document.MemberCount != 1 {
		t.Fatalf("MemberCount = %d, want 1", document.MemberCount)
	}
	if document.Rows[0].MemberName != "Pak Asep" {
		t.Errorf("MemberName = %q, want Pak Asep", document.Rows[0].MemberName)
	}
	if document.TotalPlantedHa != 2 {
		t.Errorf("TotalPlantedHa = %v, want 2: an RDKK counts what was planted this season, harvested or not",
			document.TotalPlantedHa)
	}
}

func TestRdkkLoadSeasonCarriesTheLetterheadAndSources(t *testing.T) {
	db, user := rdkkFixture(t)

	document, err := rdkkUseCase(t, db).
		LoadSeason(context.Background(), *user.CooperativeID, DefaultSeason(projectionNow))
	if err != nil {
		t.Fatalf("LoadSeason: %v", err)
	}

	if document.Meta.CooperativeName != "KUD Subang" ||
		document.Meta.Province != "Jawa Barat" {
		t.Errorf("Meta = %+v, want the cooperative identity", document.Meta)
	}
	if len(document.Sources) != 1 || document.Sources[0] != "Permentan 40/2007" {
		t.Errorf("Sources = %v, want the rate document behind the numbers", document.Sources)
	}
}

func TestRdkkLoadSeasonExcludesAnotherCooperativesPlantings(t *testing.T) {
	db, user := rdkkFixture(t)

	document, err := rdkkUseCase(t, db).
		LoadSeason(context.Background(), *user.CooperativeID, DefaultSeason(projectionNow))
	if err != nil {
		t.Fatalf("LoadSeason: %v", err)
	}

	if document.TotalPlantedHa != 2 {
		t.Errorf("TotalPlantedHa = %v, want 2: plot-other would add a third hectare",
			document.TotalPlantedHa)
	}
	for _, row := range document.Rows {
		if row.MemberName == constants.MemberWithoutName {
			t.Error("a plot outside the cooperative reached the form")
		}
	}
}

func TestRdkkCreateInputOrderStoresADraftWithNoPrices(t *testing.T) {
	db, user := rdkkFixture(t)

	created, err := rdkkUseCase(t, db).
		CreateInputOrder(context.Background(), user, projectionNow)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	if created.Lines != 2 {
		t.Errorf("Lines = %d, want 2 (urea and sp36)", created.Lines)
	}

	order := new(entity.InputOrder)
	if err := db.Where("id = ?", created.OrderID).Take(order).Error; err != nil {
		t.Fatalf("reading back the order: %v", err)
	}
	if order.Status != constants.OrderDraft {
		t.Errorf("Status = %q, want %q", order.Status, constants.OrderDraft)
	}
	if order.CooperativeID != *user.CooperativeID {
		t.Errorf("CooperativeID = %q, want the caller's", order.CooperativeID)
	}

	lines := []entity.InputOrderLine{}
	if err := db.Where("input_order_id = ?", created.OrderID).
		Order("item").Find(&lines).Error; err != nil {
		t.Fatalf("reading back the lines: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}
	if lines[0].Item != "sp36" || lines[0].Quantity != 4 {
		t.Errorf("lines[0] = %+v, want 4 sacks of sp36 from 200 kg", lines[0])
	}
	if lines[1].Item != "urea" || lines[1].Quantity != 10 {
		t.Errorf("lines[1] = %+v, want 10 sacks of urea from 500 kg", lines[1])
	}
	for _, line := range lines {
		if line.RetailPricePerUnit != nil || line.BulkPricePerUnit != nil {
			t.Errorf("line %q carries a price, want none until a supplier quotes one", line.Item)
		}
	}
}

func TestRdkkCreateInputOrderRefusesWhenThereIsNothingToOrder(t *testing.T) {
	db, user := rdkkFixture(t)
	if err := db.Exec(`DELETE FROM fertiliser_rate`).Error; err != nil {
		t.Fatalf("clearing rates: %v", err)
	}

	_, err := rdkkUseCase(t, db).CreateInputOrder(context.Background(), user, projectionNow)

	if !errors.Is(err, ErrNothingToOrder) {
		t.Errorf("err = %v, want ErrNothingToOrder", err)
	}
}

func TestRdkkCreateInputOrderRefusesAnAccountWithNoCooperative(t *testing.T) {
	db, _ := rdkkFixture(t)
	buyer := &entity.AppUser{ID: "buyer-1", Role: constants.RoleBuyer}

	_, err := rdkkUseCase(t, db).CreateInputOrder(context.Background(), buyer, projectionNow)

	if !errors.Is(err, ErrNoCooperative) {
		t.Errorf("err = %v, want ErrNoCooperative", err)
	}
}

func TestDefaultSeasonReachesBackAYear(t *testing.T) {
	season := DefaultSeason(projectionNow)

	if agronomy.DaysBetween(season.Start, season.End) != constants.RdkkSeasonDays {
		t.Errorf("season spans %d days, want %d",
			agronomy.DaysBetween(season.Start, season.End), constants.RdkkSeasonDays)
	}
	if season.Label != constants.RdkkDefaultLabel {
		t.Errorf("Label = %q, want %q", season.Label, constants.RdkkDefaultLabel)
	}
}
