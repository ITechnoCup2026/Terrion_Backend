package usecase

import (
	"context"
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

func seasonFixture(t *testing.T) *gorm.DB {
	t.Helper()

	db := seedPlanningFixture(t)
	if err := db.AutoMigrate(
		&entity.FertiliserRate{}, &entity.InputOrder{}, &entity.InputOrderLine{},
	); err != nil {
		t.Fatalf("migrating order tables: %v", err)
	}

	rates := []entity.FertiliserRate{
		{CommodityID: riceCommodity, InputItem: "urea", KgPerHa: 250, Source: "Permentan 40/2007"},
		{CommodityID: riceCommodity, InputItem: "sp36", KgPerHa: 100, Source: "Permentan 40/2007"},
	}
	if err := db.Create(&rates).Error; err != nil {
		t.Fatalf("seeding fertiliser rates: %v", err)
	}

	if err := db.Create(&entity.CooperativeCapacity{
		CooperativeID: homeCoop, CommodityID: riceCommodity, TonnesPerWeek: 1,
	}).Error; err != nil {
		t.Fatalf("seeding capacity: %v", err)
	}
	return db
}

func seasonDashboard(t *testing.T, db *gorm.DB) *DashboardUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	weatherUseCase := NewWeatherUseCase(db, log, &repository.WeatherRepository{}, nil)
	projection := NewProjectionUseCase(db, log,
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.CalibrationRepository{}, weatherUseCase)

	return NewDashboardUseCase(db, log,
		&repository.CooperativeRepository{}, &repository.BlockRepository{},
		&repository.CommodityRepository{}, &repository.MemberRepository{},
		&repository.ReferencePriceRepository{}, &repository.InputOrderRepository{},
		projection)
}

func seasonStagger(t *testing.T, db *gorm.DB) *StaggerUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	weatherUseCase := NewWeatherUseCase(db, log, &repository.WeatherRepository{}, nil)
	projection := NewProjectionUseCase(db, log,
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.CalibrationRepository{}, weatherUseCase)

	return NewStaggerUseCase(db, log, validator.New(),
		&repository.CooperativeRepository{}, &repository.BlockRepository{}, projection)
}

func seasonRdkk(t *testing.T, db *gorm.DB) *RdkkUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	return NewRdkkUseCase(db, log,
		&repository.CooperativeRepository{}, &repository.PlotRepository{},
		&repository.BlockRepository{}, &repository.MemberRepository{},
		&repository.FertiliserRateRepository{}, &repository.InputOrderRepository{})
}

func seasonCatalog(t *testing.T, db *gorm.DB) *CatalogUseCase {
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

func TestAnAppliedPlanLightsUpTheFeaturesThatWereDark(t *testing.T) {
	db := seasonFixture(t)
	planner := planningUseCase(t, db)
	user := planningManager(t, db)

	before, err := seasonDashboard(t, db).
		Load(context.Background(), homeCoop, constants.MaxHorizonWeeks, planningNow)
	if err != nil {
		t.Fatalf("dashboard before: %v", err)
	}
	if len(before.Flagged) != 0 {
		t.Fatalf("Flagged = %d before planting anything, want 0", len(before.Flagged))
	}

	proposal, err := planner.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	applied, err := planner.Apply(
		context.Background(), user, firstPlanRequest(t, proposal), planningNow)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	after, err := seasonDashboard(t, db).
		Load(context.Background(), homeCoop, constants.MaxHorizonWeeks, planningNow)
	if err != nil {
		t.Fatalf("dashboard after: %v", err)
	}

	projected := 0.0
	for _, week := range after.Weeks {
		projected += week.ExpectedTonnes
	}
	if projected <= 0 {
		t.Error("the dashboard projects no tonnage for next season, want the plan to fill it")
	}

	if len(after.Flagged) == 0 {
		t.Fatal("no harvest collision is flagged, want the plan to make one visible")
	}
	if len(after.Suggestions) == 0 {
		t.Fatal("no stagger suggestion is offered, want at least one")
	}

	suggestion := after.Suggestions[0]
	shifted, err := seasonStagger(t, db).Apply(context.Background(), user,
		&model.ApplyStaggerRequest{
			ISOWeek:     suggestion.ISOWeek,
			CommodityID: suggestion.CommodityID,
		}, planningNow)
	if err != nil {
		t.Fatalf("staggering a planned week: %v, want it to succeed now that the "+
			"blocks are not planted yet", err)
	}
	if shifted.Shifted == 0 {
		t.Error("Shifted = 0, want the stagger to move at least one block")
	}

	order, err := seasonRdkk(t, db).CreateInputOrder(context.Background(), user, Season{
		Label: proposal.Season.Label,
		Start: proposal.Season.Start,
		End:   proposal.Season.End,
	})
	if err != nil {
		t.Fatalf("creating next season's input order: %v", err)
	}
	if order.Lines == 0 {
		t.Error("Lines = 0, want next season's fertiliser needs to be issued before it starts")
	}

	listings, err := seasonCatalog(t, db).LoadForCooperative(
		context.Background(), homeCoop, constants.MaxHorizonWeeks, planningNow)
	if err != nil {
		t.Fatalf("loading the public catalogue: %v", err)
	}
	if len(listings) == 0 {
		t.Error("the catalogue lists nothing, want next season's harvest windows")
	}

	if err := db.Create(&entity.Block{
		ID: "block-kader-integration", PlotID: "plot-1", Label: "BLOK Z",
		AreaHa: 0.5, OrderIndex: 9,
		CommodityID: riceCommodity, VarietyID: "variety-rice",
		PlantingDate: agronomy.AddDays(planningNow, -30),
	}).Error; err != nil {
		t.Fatalf("seeding a field record: %v", err)
	}

	detail, err := plotUseCase(t, db).
		Get(context.Background(), homeCoop, "plot-1", planningNow)
	if err != nil {
		t.Fatalf("reading plot-1: %v", err)
	}

	fromPlan, fromKader := 0, 0
	for _, block := range detail.Blocks {
		if block.SeasonPlanID == nil {
			fromKader++
			continue
		}
		if *block.SeasonPlanID != applied.PlanID {
			t.Errorf("block %s points at another plan", block.ID)
		}
		fromPlan++
	}
	if fromPlan == 0 || fromKader == 0 {
		t.Errorf("plot-1 shows %d planned and %d recorded blocks, want both kinds "+
			"distinguishable on the same screen", fromPlan, fromKader)
	}

	cancelled, err := planner.Cancel(context.Background(), user, applied.PlanID, planningNow)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if cancelled.Blocks == 0 {
		t.Error("Blocks = 0, want the cancellation to remove the planned blocks")
	}

	remaining := []entity.Block{}
	if err := db.Where("season_plan_id IS NOT NULL").Find(&remaining).Error; err != nil {
		t.Fatalf("reading blocks after cancelling: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("%d planned blocks survived the cancellation, want none", len(remaining))
	}

	kader := new(entity.Block)
	if err := db.Where("id = ?", "block-kader-integration").Take(kader).Error; err != nil {
		t.Fatalf("the field record was deleted with the plan: %v", err)
	}
}
