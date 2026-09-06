package usecase

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
)

func rdkkUseCase(t *testing.T, db *gorm.DB) *RdkkUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	return NewRdkkUseCase(db, log, validator.New(),
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
		{CommodityID: maizeCommodity, InputItem: "urea", KgPerHa: 250, Source: "Permentan 40/2007"},
		{CommodityID: maizeCommodity, InputItem: "sp36", KgPerHa: 100, Source: "Permentan 40/2007"},
	}
	if err := db.Create(&rates).Error; err != nil {
		t.Fatalf("seeding fertiliser rates: %v", err)
	}

	cooperativeID := homeCoop
	return db, &entity.AppUser{
		ID: "pengurus-1", FullName: "Budi Santoso",
		Role: constants.RolePengurus, CooperativeID: &cooperativeID,
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
		CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)
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

	_, err := rdkkUseCase(t, db).CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)

	if !errors.Is(err, ErrNothingToOrder) {
		t.Errorf("err = %v, want ErrNothingToOrder", err)
	}
}

func TestRdkkCreateInputOrderRefusesAnAccountWithNoCooperative(t *testing.T) {
	db, _ := rdkkFixture(t)
	buyer := &entity.AppUser{ID: "buyer-1", Role: constants.RoleBuyer}

	_, err := rdkkUseCase(t, db).CreateInputOrder(context.Background(), buyer, DefaultSeason(projectionNow), nil)

	if !errors.Is(err, ErrNoCooperative) {
		t.Errorf("err = %v, want ErrNoCooperative", err)
	}
}

func TestRdkkListInputOrdersReturnsTheOrderWithItsLines(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)

	created, err := useCase.CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	orders, err := useCase.ListInputOrders(context.Background(), user)
	if err != nil {
		t.Fatalf("ListInputOrders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("len(orders) = %d, want 1", len(orders))
	}
	if orders[0].Order.ID != created.OrderID {
		t.Errorf("Order.ID = %q, want %q", orders[0].Order.ID, created.OrderID)
	}
	if orders[0].Order.Status != constants.OrderDraft {
		t.Errorf("Status = %q, want %q", orders[0].Order.Status, constants.OrderDraft)
	}
	if len(orders[0].Lines) != 2 {
		t.Errorf("len(Lines) = %d, want 2", len(orders[0].Lines))
	}
}

func TestRdkkListInputOrdersExcludesAnotherCooperatives(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)

	if _, err := useCase.CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil); err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	stranger := otherCoop
	orders, err := useCase.ListInputOrders(
		context.Background(), &entity.AppUser{ID: "pengurus-2", Role: constants.RolePengurus, CooperativeID: &stranger})
	if err != nil {
		t.Fatalf("ListInputOrders: %v", err)
	}
	if len(orders) != 0 {
		t.Errorf("len(orders) = %d, want 0 for another cooperative", len(orders))
	}
}

func TestRdkkListInputOrdersRefusesAnAccountWithNoCooperative(t *testing.T) {
	db, _ := rdkkFixture(t)
	buyer := &entity.AppUser{ID: "buyer-1", Role: constants.RoleBuyer}

	_, err := rdkkUseCase(t, db).ListInputOrders(context.Background(), buyer)
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

func TestRdkkCreateInputOrderCoversTheSeasonItWasGiven(t *testing.T) {
	db, user := rdkkFixture(t)

	nextSeason := Season{
		Label: "MT I 2026/2027",
		Start: agronomy.AddDays(projectionNow, 1),
		End:   agronomy.AddDays(projectionNow, 210),
	}

	_, err := rdkkUseCase(t, db).CreateInputOrder(context.Background(), user, nextSeason, nil)
	if !errors.Is(err, ErrNothingToOrder) {
		t.Fatalf("err = %v, want %v before anything is planted for that season", err, ErrNothingToOrder)
	}

	if err := db.Create(&entity.Block{
		ID: "block-next-season", PlotID: "plot-home", Label: "BLOK R", AreaHa: 2, OrderIndex: 7,
		CommodityID: maizeCommodity, VarietyID: "variety-1",
		PlantingDate: agronomy.AddDays(projectionNow, 30),
	}).Error; err != nil {
		t.Fatalf("seeding a planned block: %v", err)
	}

	created, err := rdkkUseCase(t, db).CreateInputOrder(context.Background(), user, nextSeason, nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}
	if created.Lines != 2 {
		t.Errorf("Lines = %d, want 2 (urea and sp36) for next season", created.Lines)
	}

	order := new(entity.InputOrder)
	if err := db.Where("id = ?", created.OrderID).Take(order).Error; err != nil {
		t.Fatalf("reading back the order: %v", err)
	}
	if order.SeasonLabel != nextSeason.Label {
		t.Errorf("SeasonLabel = %q, want %q", order.SeasonLabel, nextSeason.Label)
	}
}

func TestRdkkCreateInputOrderRefusesASecondOrderForTheSameSeason(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)
	season := DefaultSeason(projectionNow)

	if _, err := useCase.CreateInputOrder(context.Background(), user, season, nil); err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	_, err := useCase.CreateInputOrder(context.Background(), user, season, nil)
	if !errors.Is(err, ErrOrderSeasonAlreadyOpen) {
		t.Errorf("err = %v, want ErrOrderSeasonAlreadyOpen", err)
	}
}

func TestRdkkCreateInputOrderAllowsANewOrderOnceTheLastIsFinished(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)
	season := DefaultSeason(projectionNow)

	first, err := useCase.CreateInputOrder(context.Background(), user, season, nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	submitted := &model.UpdateInputOrderStatusRequest{Status: constants.OrderSubmitted}
	if err := useCase.UpdateInputOrderStatus(
		context.Background(), user, first.OrderID, submitted, projectionNow); err != nil {
		t.Fatalf("UpdateInputOrderStatus to submitted: %v", err)
	}
	completed := &model.UpdateInputOrderStatusRequest{Status: constants.OrderCompleted}
	if err := useCase.UpdateInputOrderStatus(
		context.Background(), user, first.OrderID, completed, projectionNow); err != nil {
		t.Fatalf("UpdateInputOrderStatus to completed: %v", err)
	}

	if _, err := useCase.CreateInputOrder(context.Background(), user, season, nil); err != nil {
		t.Errorf("CreateInputOrder after completion: %v, want it allowed", err)
	}
}

func TestRdkkCreateInputOrderRecordsWhoMadeIt(t *testing.T) {
	db, user := rdkkFixture(t)

	created, err := rdkkUseCase(t, db).
		CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	order := new(entity.InputOrder)
	if err := db.Where("id = ?", created.OrderID).Take(order).Error; err != nil {
		t.Fatalf("reading back the order: %v", err)
	}
	if order.CreatedByID == nil || *order.CreatedByID != user.ID {
		t.Errorf("CreatedByID = %v, want %q", order.CreatedByID, user.ID)
	}
	if order.CreatedByName == nil || *order.CreatedByName != user.FullName {
		t.Errorf("CreatedByName = %v, want %q", order.CreatedByName, user.FullName)
	}
	if order.StatusChangedAt != nil {
		t.Errorf("StatusChangedAt = %v, want nil until the status actually moves", order.StatusChangedAt)
	}
}

func TestRdkkCreateInputOrderKeepsTheRdkkFigureBesideAnAdjustedOne(t *testing.T) {
	db, user := rdkkFixture(t)

	request := &model.CreateInputOrderRequest{
		Lines: []model.InputOrderLineRequest{{Item: "urea", Quantity: 6}},
	}
	created, err := rdkkUseCase(t, db).
		CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), request)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	lines := []entity.InputOrderLine{}
	if err := db.Where("input_order_id = ?", created.OrderID).
		Order("item").Find(&lines).Error; err != nil {
		t.Fatalf("reading back the lines: %v", err)
	}

	var urea, sp36 *entity.InputOrderLine
	for i := range lines {
		switch lines[i].Item {
		case "urea":
			urea = &lines[i]
		case "sp36":
			sp36 = &lines[i]
		}
	}
	if urea == nil || urea.Quantity != 6 || urea.QuantityRdkk == nil || *urea.QuantityRdkk != 10 {
		t.Errorf("urea line = %+v, want quantity 6 with quantity_rdkk 10", urea)
	}
	if sp36 == nil || sp36.Quantity != 4 || sp36.QuantityRdkk != nil {
		t.Errorf("sp36 line = %+v, want quantity 4 untouched with no quantity_rdkk", sp36)
	}
}

func TestRdkkCreateInputOrderDropsALineAdjustedToNothing(t *testing.T) {
	db, user := rdkkFixture(t)

	request := &model.CreateInputOrderRequest{
		Lines: []model.InputOrderLineRequest{{Item: "sp36", Quantity: 0}},
	}
	created, err := rdkkUseCase(t, db).
		CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), request)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}
	if created.Lines != 1 {
		t.Errorf("Lines = %d, want 1: sp36 zeroed out should be dropped, not stored as 0", created.Lines)
	}

	lines := []entity.InputOrderLine{}
	if err := db.Where("input_order_id = ?", created.OrderID).Find(&lines).Error; err != nil {
		t.Fatalf("reading back the lines: %v", err)
	}
	for _, line := range lines {
		if line.Item == "sp36" {
			t.Error("sp36 should have been dropped, not stored as a zero-quantity line")
		}
	}
}

func TestRdkkCreateInputOrderRefusesAnItemTheSeasonNeverAskedFor(t *testing.T) {
	db, user := rdkkFixture(t)

	request := &model.CreateInputOrderRequest{
		Lines: []model.InputOrderLineRequest{{Item: "kcl", Quantity: 5}},
	}
	_, err := rdkkUseCase(t, db).
		CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), request)
	if !errors.Is(err, ErrOrderLineUnknown) {
		t.Errorf("err = %v, want ErrOrderLineUnknown", err)
	}
}

func TestRdkkCreateInputOrderRefusesAnOrderAdjustedDownToNothing(t *testing.T) {
	db, user := rdkkFixture(t)

	request := &model.CreateInputOrderRequest{
		Lines: []model.InputOrderLineRequest{
			{Item: "urea", Quantity: 0}, {Item: "sp36", Quantity: 0},
		},
	}
	_, err := rdkkUseCase(t, db).
		CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), request)
	if !errors.Is(err, ErrOrderLinesEmpty) {
		t.Errorf("err = %v, want ErrOrderLinesEmpty", err)
	}
}

func TestRdkkCreateInputOrderRefusesANegativeAdjustment(t *testing.T) {
	db, user := rdkkFixture(t)

	request := &model.CreateInputOrderRequest{
		Lines: []model.InputOrderLineRequest{{Item: "urea", Quantity: -1}},
	}
	_, err := rdkkUseCase(t, db).
		CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), request)

	var validationError validator.ValidationErrors
	if !errors.As(err, &validationError) {
		t.Errorf("err = %v, want a validation error for a negative quantity", err)
	}
}

func TestRdkkUpdateInputOrderStatusRecordsWhoMovedItAndWhen(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)

	created, err := useCase.CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	request := &model.UpdateInputOrderStatusRequest{Status: constants.OrderSubmitted}
	if err := useCase.UpdateInputOrderStatus(
		context.Background(), user, created.OrderID, request, projectionNow); err != nil {
		t.Fatalf("UpdateInputOrderStatus: %v", err)
	}

	order := new(entity.InputOrder)
	if err := db.Where("id = ?", created.OrderID).Take(order).Error; err != nil {
		t.Fatalf("reading back the order: %v", err)
	}
	if order.Status != constants.OrderSubmitted {
		t.Errorf("Status = %q, want %q", order.Status, constants.OrderSubmitted)
	}
	if order.StatusChangedByID == nil || *order.StatusChangedByID != user.ID {
		t.Errorf("StatusChangedByID = %v, want %q", order.StatusChangedByID, user.ID)
	}
	if order.StatusChangedByName == nil || *order.StatusChangedByName != user.FullName {
		t.Errorf("StatusChangedByName = %v, want %q", order.StatusChangedByName, user.FullName)
	}
	if order.StatusChangedAt == nil || !order.StatusChangedAt.Equal(projectionNow.UTC()) {
		t.Errorf("StatusChangedAt = %v, want %v", order.StatusChangedAt, projectionNow.UTC())
	}
}

func TestRdkkUpdateInputOrderStatusRefusesSkippingSubmission(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)

	created, err := useCase.CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	request := &model.UpdateInputOrderStatusRequest{Status: constants.OrderCompleted}
	err = useCase.UpdateInputOrderStatus(context.Background(), user, created.OrderID, request, projectionNow)
	if !errors.Is(err, ErrOrderTransitionInvalid) {
		t.Errorf("err = %v, want ErrOrderTransitionInvalid", err)
	}
}

func TestRdkkUpdateInputOrderStatusRefusesLeavingAFinishedOrder(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)

	created, err := useCase.CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	cancel := &model.UpdateInputOrderStatusRequest{Status: constants.OrderCancelled}
	if err := useCase.UpdateInputOrderStatus(
		context.Background(), user, created.OrderID, cancel, projectionNow); err != nil {
		t.Fatalf("cancelling the order: %v", err)
	}

	submit := &model.UpdateInputOrderStatusRequest{Status: constants.OrderSubmitted}
	err = useCase.UpdateInputOrderStatus(context.Background(), user, created.OrderID, submit, projectionNow)
	if !errors.Is(err, ErrOrderAlreadyFinal) {
		t.Errorf("err = %v, want ErrOrderAlreadyFinal", err)
	}
}

func TestRdkkUpdateInputOrderStatusRefusesAnotherCooperativesOrder(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)

	created, err := useCase.CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	stranger := otherCoop
	strangerUser := &entity.AppUser{ID: "pengurus-2", Role: constants.RolePengurus, CooperativeID: &stranger}
	request := &model.UpdateInputOrderStatusRequest{Status: constants.OrderSubmitted}
	err = useCase.UpdateInputOrderStatus(context.Background(), strangerUser, created.OrderID, request, projectionNow)
	if !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("err = %v, want ErrOrderNotFound", err)
	}
}

func TestRdkkUpdateInputOrderStatusRefusesAStatusThatIsNotAStep(t *testing.T) {
	db, user := rdkkFixture(t)
	useCase := rdkkUseCase(t, db)

	created, err := useCase.CreateInputOrder(context.Background(), user, DefaultSeason(projectionNow), nil)
	if err != nil {
		t.Fatalf("CreateInputOrder: %v", err)
	}

	request := &model.UpdateInputOrderStatusRequest{Status: constants.OrderDraft}
	err = useCase.UpdateInputOrderStatus(context.Background(), user, created.OrderID, request, projectionNow)

	var validationError validator.ValidationErrors
	if !errors.As(err, &validationError) {
		t.Errorf("err = %v, want a validation error: draft is not in the oneof enum", err)
	}
}
