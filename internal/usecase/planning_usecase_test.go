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
		projection, weatherUseCase)
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
