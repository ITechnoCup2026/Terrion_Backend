package usecase

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

var (
	planningNow   = time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	planSeason    = "MT I 2026/2027"
	riceCommodity = "55555555-5555-4555-8555-555555555555"
)

func planningDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := projectionDB(t)
	if err := db.AutoMigrate(
		&entity.Cooperative{}, &entity.CooperativeCapacity{}, &entity.Member{},
		&entity.Commodity{}, &entity.ReferencePrice{}, &entity.SupplyContractRequest{},
		&entity.AppUser{}, &entity.SeasonPlan{}, &entity.SeasonPlanItem{},
	); err != nil {
		t.Fatalf("migrating planning tables: %v", err)
	}
	return db
}

func planningUseCase(t *testing.T, db *gorm.DB) *PlanningUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	weatherUseCase := NewWeatherUseCase(db, log, &repository.WeatherRepository{}, nil)
	projection := NewProjectionUseCase(db, log,
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.CalibrationRepository{}, weatherUseCase)

	return NewPlanningUseCase(db, log, validator.New(),
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.MemberRepository{}, &repository.CommodityRepository{},
		&repository.VarietyRepository{}, &repository.CooperativeRepository{},
		&repository.ReferencePriceRepository{}, &repository.SupplyRequestRepository{},
		&repository.SeasonPlanRepository{}, projection, weatherUseCase, nil)
}

func seedPlanningFixture(t *testing.T) *gorm.DB {
	t.Helper()

	db := planningDB(t)
	seedProjectionWeather(t, db, homeCell)

	coop := &entity.Cooperative{
		ID: homeCoop, Name: "KUD Uji", Village: "Desa", District: "Kabupaten",
		Province: "Jawa Barat", Lat: -6.25, Lng: 107.75, CreatedAt: planningNow,
	}
	if err := db.Create(coop).Error; err != nil {
		t.Fatalf("seeding cooperative: %v", err)
	}

	if err := db.Create(&entity.Commodity{
		ID: riceCommodity, Slug: "padi", Name: "Padi",
	}).Error; err != nil {
		t.Fatalf("seeding commodity: %v", err)
	}

	if err := db.Create(&entity.Variety{
		ID: "variety-rice", CommodityID: riceCommodity, Name: "Ciherang",
		GddRequirement: 1860, BaseTempC: 12,
		DaysToHarvestMin: 110, DaysToHarvestMax: 125,
		YieldPerHaMin: 5, YieldPerHaMax: 7,
	}).Error; err != nil {
		t.Fatalf("seeding variety: %v", err)
	}

	for _, id := range []string{"plot-1", "plot-2", "plot-3"} {
		if err := db.Create(&entity.Member{
			ID: "member-" + id, CooperativeID: homeCoop,
			Name: "Anggota " + id, CreatedAt: planningNow,
		}).Error; err != nil {
			t.Fatalf("seeding member %s: %v", id, err)
		}
		seedPlot(t, db, id, homeCoop, homeCell)
	}
	return db
}

func TestProposeReturnsThreePlansCoveringEveryFreePlot(t *testing.T) {
	db := seedPlanningFixture(t)

	proposal, err := planningUseCase(t, db).
		Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if len(proposal.Plans) != 3 {
		t.Fatalf("len(Plans) = %d, want 3", len(proposal.Plans))
	}
	for _, plan := range proposal.Plans {
		if len(plan.Assignments) != 3 {
			t.Errorf("plan %q covers %d plots, want 3", plan.Objective, len(plan.Assignments))
		}
	}
	if len(proposal.Skipped) != 0 {
		t.Errorf("Skipped = %+v, want empty", proposal.Skipped)
	}
}

func TestProposeNeverPlantsBeforeToday(t *testing.T) {
	db := seedPlanningFixture(t)

	proposal, err := planningUseCase(t, db).
		Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	for _, plan := range proposal.Plans {
		for _, assignment := range plan.Assignments {
			if !assignment.PlantingDate.After(planningNow) {
				t.Errorf("plot %s planted %s, which is not after %s",
					assignment.PlotID,
					agronomy.ToISODate(assignment.PlantingDate),
					agronomy.ToISODate(planningNow))
			}
		}
	}
}

func TestProposeSkipsAPlotStillCarryingACrop(t *testing.T) {
	db := seedPlanningFixture(t)

	if err := db.Create(&entity.Block{
		ID: "block-occupied", PlotID: "plot-2", Label: "BLOK A", AreaHa: 1,
		CommodityID: riceCommodity, VarietyID: "variety-rice",
		PlantingDate: agronomy.AddDays(planningNow, 60),
	}).Error; err != nil {
		t.Fatalf("seeding occupied block: %v", err)
	}

	proposal, err := planningUseCase(t, db).
		Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if len(proposal.Skipped) != 1 || proposal.Skipped[0].PlotID != "plot-2" {
		t.Fatalf("Skipped = %+v, want exactly plot-2", proposal.Skipped)
	}
	for _, plan := range proposal.Plans {
		for _, assignment := range plan.Assignments {
			if assignment.PlotID == "plot-2" {
				t.Errorf("plan %q assigned the occupied plot", plan.Objective)
			}
		}
	}
}

func TestProposeRefusesWhenTheCooperativeHasNoPlots(t *testing.T) {
	db := planningDB(t)
	seedProjectionWeather(t, db, homeCell)

	_, err := planningUseCase(t, db).
		Propose(context.Background(), homeCoop, planSeason, planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanNoPlots {
		t.Fatalf("err = %v, want %s", err, constants.PlanNoPlots)
	}
}

func TestProposeRefusesWithoutClimateNormals(t *testing.T) {
	db := seedPlanningFixture(t)
	if err := db.Where("1 = 1").Delete(&entity.WeatherNormal{}).Error; err != nil {
		t.Fatalf("clearing normals: %v", err)
	}

	_, err := planningUseCase(t, db).
		Propose(context.Background(), homeCoop, planSeason, planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanNoClimateNormals {
		t.Fatalf("err = %v, want %s", err, constants.PlanNoClimateNormals)
	}
}

func TestProposeRefusesASeasonWhosePlantingWindowClosed(t *testing.T) {
	db := seedPlanningFixture(t)

	_, err := planningUseCase(t, db).
		Propose(context.Background(), homeCoop, "MT I 2019/2020", planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanSeasonClosed {
		t.Fatalf("err = %v, want %s", err, constants.PlanSeasonClosed)
	}
}

func TestProposeIsDeterministic(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)

	first, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("first Propose: %v", err)
	}
	second, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("second Propose: %v", err)
	}

	for p := range first.Plans {
		for a := range first.Plans[p].Assignments {
			if first.Plans[p].Assignments[a] != second.Plans[p].Assignments[a] {
				t.Fatalf("plan %d assignment %d differs", p, a)
			}
		}
	}
}

var _ = weather.GridCell{}

func planningManager(t *testing.T, db *gorm.DB) *entity.AppUser {
	t.Helper()

	coopID := homeCoop
	user := &entity.AppUser{
		ID: "99999999-9999-4999-8999-999999999999", Role: constants.RolePengurus,
		CooperativeID: &coopID, FullName: "Pengurus Uji", CreatedAt: planningNow,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seeding manager: %v", err)
	}
	return user
}

func firstPlanRequest(t *testing.T, proposal Proposal) *model.ApplySeasonPlanRequest {
	t.Helper()

	plan := proposal.Plans[0]
	assignments := make([]model.SeasonPlanAssignmentRequest, len(plan.Assignments))
	for i, assignment := range plan.Assignments {
		assignments[i] = model.SeasonPlanAssignmentRequest{
			PlotID:       assignment.PlotID,
			VarietyID:    assignment.VarietyID,
			PlantingDate: agronomy.ToISODate(assignment.PlantingDate),
		}
	}

	return &model.ApplySeasonPlanRequest{
		SeasonLabel: proposal.Season.Label,
		Objective:   string(plan.Objective),
		Assignments: assignments,
	}
}

func TestApplyCreatesBlocksMarkedWithThePlan(t *testing.T) {
	db := seedPlanningFixture(t)
	user := planningManager(t, db)
	useCase := planningUseCase(t, db)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	applied, err := useCase.Apply(
		context.Background(), user, firstPlanRequest(t, proposal), planningNow)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.Blocks != 3 {
		t.Fatalf("Blocks = %d, want 3", applied.Blocks)
	}

	blocks := []entity.Block{}
	if err := db.Where("season_plan_id = ?", applied.PlanID).Find(&blocks).Error; err != nil {
		t.Fatalf("reading plan blocks: %v", err)
	}
	if len(blocks) != 3 {
		t.Fatalf("len(blocks) = %d, want 3", len(blocks))
	}
	for _, block := range blocks {
		if !block.PlantingDate.After(planningNow) {
			t.Errorf("block %s planted %s, want a future date",
				block.ID, agronomy.ToISODate(block.PlantingDate))
		}
	}

	items := []entity.SeasonPlanItem{}
	if err := db.Where("plan_id = ?", applied.PlanID).Find(&items).Error; err != nil {
		t.Fatalf("reading plan items: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("len(items) = %d, want 3", len(items))
	}
	for _, item := range items {
		if item.BlockID == nil {
			t.Errorf("item %s has no block_id", item.ID)
		}
		if item.ExpectedTonnesMid <= 0 {
			t.Errorf("item %s stored no expected tonnage", item.ID)
		}
	}
}

func TestApplyRejectsAPlotFromAnotherCooperative(t *testing.T) {
	db := seedPlanningFixture(t)
	user := planningManager(t, db)
	useCase := planningUseCase(t, db)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	request := firstPlanRequest(t, proposal)
	request.Assignments[0].PlotID = "plot-of-another-cooperative"

	_, err = useCase.Apply(context.Background(), user, request, planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanAssignmentRejected {
		t.Fatalf("err = %v, want %s", err, constants.PlanAssignmentRejected)
	}
}

func TestApplyRejectsAPlantingDateInThePast(t *testing.T) {
	db := seedPlanningFixture(t)
	user := planningManager(t, db)
	useCase := planningUseCase(t, db)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	request := firstPlanRequest(t, proposal)
	request.Assignments[0].PlantingDate = agronomy.ToISODate(
		agronomy.AddDays(planningNow, -7))

	_, err = useCase.Apply(context.Background(), user, request, planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanAssignmentRejected {
		t.Fatalf("err = %v, want %s", err, constants.PlanAssignmentRejected)
	}
}

func TestApplyRecomputesTonnageInsteadOfTrustingTheClient(t *testing.T) {
	db := seedPlanningFixture(t)
	user := planningManager(t, db)
	useCase := planningUseCase(t, db)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	applied, err := useCase.Apply(
		context.Background(), user, firstPlanRequest(t, proposal), planningNow)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	items := []entity.SeasonPlanItem{}
	if err := db.Where("plan_id = ?", applied.PlanID).
		Order("plot_id").Find(&items).Error; err != nil {
		t.Fatalf("reading plan items: %v", err)
	}

	expected := map[string]float64{}
	for _, assignment := range proposal.Plans[0].Assignments {
		expected[assignment.PlotID] = assignment.TonnesMid
	}
	for _, item := range items {
		if item.ExpectedTonnesMid != expected[item.PlotID] {
			t.Errorf("plot %s stored %v, want the server figure %v",
				item.PlotID, item.ExpectedTonnesMid, expected[item.PlotID])
		}
	}
}

func TestApplyTwiceForTheSameSeasonRefuses(t *testing.T) {
	db := seedPlanningFixture(t)
	user := planningManager(t, db)
	useCase := planningUseCase(t, db)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if _, err := useCase.Apply(
		context.Background(), user, firstPlanRequest(t, proposal), planningNow); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	_, err = useCase.Apply(
		context.Background(), user, firstPlanRequest(t, proposal), planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanAlreadyApplied {
		t.Fatalf("err = %v, want %s", err, constants.PlanAlreadyApplied)
	}
}

func applyFirstPlan(t *testing.T, db *gorm.DB) (*PlanningUseCase, *entity.AppUser, AppliedPlan) {
	t.Helper()

	user := planningManager(t, db)
	useCase := planningUseCase(t, db)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	applied, err := useCase.Apply(
		context.Background(), user, firstPlanRequest(t, proposal), planningNow)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return useCase, user, applied
}

func TestCancelRemovesPlanBlocksAndKeepsFieldRecords(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase, user, applied := applyFirstPlan(t, db)

	if err := db.Create(&entity.Block{
		ID: "block-kader", PlotID: "plot-1", Label: "BLOK Z", AreaHa: 0.5, OrderIndex: 9,
		CommodityID: riceCommodity, VarietyID: "variety-rice",
		PlantingDate: agronomy.AddDays(planningNow, -30),
	}).Error; err != nil {
		t.Fatalf("seeding field record: %v", err)
	}

	cancelled, err := useCase.Cancel(context.Background(), user, applied.PlanID, planningNow)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if cancelled.Blocks != 3 {
		t.Errorf("Blocks = %d, want 3", cancelled.Blocks)
	}

	remaining := []entity.Block{}
	if err := db.Where("season_plan_id = ?", applied.PlanID).Find(&remaining).Error; err != nil {
		t.Fatalf("reading plan blocks: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("len(remaining) = %d, want 0", len(remaining))
	}

	kader := new(entity.Block)
	if err := db.Where("id = ?", "block-kader").Take(kader).Error; err != nil {
		t.Fatalf("the field record was deleted with the plan: %v", err)
	}

	plan := new(entity.SeasonPlan)
	if err := db.Where("id = ?", applied.PlanID).Take(plan).Error; err != nil {
		t.Fatalf("reading plan: %v", err)
	}
	if plan.Status != constants.PlanCancelled {
		t.Errorf("Status = %q, want %q", plan.Status, constants.PlanCancelled)
	}
	if plan.CancelledAt == nil {
		t.Error("CancelledAt = nil, want a timestamp")
	}

	items := []entity.SeasonPlanItem{}
	if err := db.Where("plan_id = ?", applied.PlanID).Find(&items).Error; err != nil {
		t.Fatalf("reading plan items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want the plan to stay readable after cancelling", len(items))
	}
	for _, item := range items {
		if item.BlockID != nil {
			t.Errorf("item %s still points at a deleted block", item.ID)
		}
	}
}

func TestCancelTwiceRefuses(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase, user, applied := applyFirstPlan(t, db)

	if _, err := useCase.Cancel(
		context.Background(), user, applied.PlanID, planningNow); err != nil {
		t.Fatalf("first Cancel: %v", err)
	}

	_, err := useCase.Cancel(context.Background(), user, applied.PlanID, planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanAlreadyCancelled {
		t.Fatalf("err = %v, want %s", err, constants.PlanAlreadyCancelled)
	}
}

func TestCancelRefusesAPlanOfAnotherCooperative(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase, _, applied := applyFirstPlan(t, db)

	other := otherCoop
	stranger := &entity.AppUser{
		ID: "88888888-8888-4888-8888-888888888888", Role: constants.RolePengurus,
		CooperativeID: &other, FullName: "Pengurus Lain", CreatedAt: planningNow,
	}
	if err := db.Create(stranger).Error; err != nil {
		t.Fatalf("seeding the other manager: %v", err)
	}

	_, err := useCase.Cancel(context.Background(), stranger, applied.PlanID, planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanNotFound {
		t.Fatalf("err = %v, want %s", err, constants.PlanNotFound)
	}
}

func TestCancelRefusesOnceAPlanBlockCarriesAHarvest(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase, user, applied := applyFirstPlan(t, db)

	harvested := agronomy.AddDays(planningNow, 200)
	if err := db.Model(&entity.Block{}).
		Where("season_plan_id = ?", applied.PlanID).
		Limit(1).
		Update("actual_harvest_date", harvested).Error; err != nil {
		t.Fatalf("recording a harvest on a plan block: %v", err)
	}

	_, err := useCase.Cancel(context.Background(), user, applied.PlanID, planningNow)

	refusal := new(PlanRefusal)
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanPartiallyCancellable {
		t.Fatalf("err = %v, want %s", err, constants.PlanPartiallyCancellable)
	}

	remaining := []entity.Block{}
	if err := db.Where("season_plan_id = ?", applied.PlanID).Find(&remaining).Error; err != nil {
		t.Fatalf("reading plan blocks: %v", err)
	}
	if len(remaining) != 3 {
		t.Errorf("len(remaining) = %d, want the refusal to change nothing", len(remaining))
	}
}
