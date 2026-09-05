package usecase

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/planning"
	"terrion-backend/internal/weather"
)

type statementCounter struct {
	gormlogger.Interface
	count atomic.Int64
}

func (c *statementCounter) Trace(
	ctx context.Context, begin time.Time,
	fc func() (string, int64), err error,
) {
	c.count.Add(1)
}

func countingSession(db *gorm.DB) (*gorm.DB, *statementCounter) {
	counter := &statementCounter{Interface: gormlogger.Discard}
	return db.Session(&gorm.Session{Logger: counter, NewDB: true}), counter
}

func planAssignments(n int) []planning.Assignment {
	assignments := make([]planning.Assignment, n)
	for i := range assignments {
		assignments[i] = planning.Assignment{
			PlotID:       "plot-1",
			MemberID:     "member-plot-1",
			AreaHa:       1,
			CommodityID:  riceCommodity,
			VarietyID:    "variety-rice",
			PlantingDate: agronomy.AddDays(planningNow, 10+i),
			Window: agronomy.DateRange{
				Start: agronomy.AddDays(planningNow, 120+i),
				End:   agronomy.AddDays(planningNow, 135+i),
			},
			Plausibility: constants.PlausibilityOk,
			TonnesLow:    5, TonnesMid: 6, TonnesHigh: 7,
		}
	}
	return assignments
}

func persistWithCount(t *testing.T, n int) int64 {
	t.Helper()

	db := seedPlanningFixture(t)
	user := planningManager(t, db)
	useCase := planningUseCase(t, db)

	session, counter := countingSession(db)
	useCase.DB = session

	plan := &entity.SeasonPlan{
		ID:            "plan-" + time.Now().Format("150405.000000000"),
		CooperativeID: homeCoop,
		SeasonLabel:   planSeason,
		SeasonStart:   planningNow,
		SeasonEnd:     agronomy.AddDays(planningNow, 180),
		Objective:     constants.ObjectiveSafe,
		Status:        constants.PlanApplied,
		CreatedBy:     user.ID,
		CreatedAt:     planningNow,
	}
	if err := useCase.persistPlan(context.Background(), plan, planAssignments(n)); err != nil {
		t.Fatalf("persistPlan(%d assignments): %v", n, err)
	}
	return counter.count.Load()
}

func TestPersistPlanIssuesAConstantNumberOfStatements(t *testing.T) {
	one := persistWithCount(t, 1)
	many := persistWithCount(t, 12)

	if many != one {
		t.Errorf("statements = %d for 12 assignments and %d for 1; "+
			"persistPlan must not scale its round trips with the assignment count",
			many, one)
	}
}

func TestPersistPlanNumbersBlocksSequentiallyOnTheSamePlot(t *testing.T) {
	db := seedPlanningFixture(t)
	user := planningManager(t, db)
	useCase := planningUseCase(t, db)

	plan := &entity.SeasonPlan{
		ID:            "plan-sequential",
		CooperativeID: homeCoop,
		SeasonLabel:   planSeason,
		SeasonStart:   planningNow,
		SeasonEnd:     agronomy.AddDays(planningNow, 180),
		Objective:     constants.ObjectiveSafe,
		Status:        constants.PlanApplied,
		CreatedBy:     user.ID,
		CreatedAt:     planningNow,
	}
	if err := useCase.persistPlan(context.Background(), plan, planAssignments(3)); err != nil {
		t.Fatalf("persistPlan: %v", err)
	}

	blocks := []entity.Block{}
	if err := db.Where("season_plan_id = ?", plan.ID).
		Order("order_index").Find(&blocks).Error; err != nil {
		t.Fatalf("reading plan blocks: %v", err)
	}
	if len(blocks) != 3 {
		t.Fatalf("len(blocks) = %d, want 3", len(blocks))
	}

	seen := map[int]bool{}
	for _, block := range blocks {
		if seen[block.OrderIndex] {
			t.Errorf("order_index %d used twice on plot %s", block.OrderIndex, block.PlotID)
		}
		seen[block.OrderIndex] = true
	}
}

func TestNormalsForReadsEveryCellInOneStatement(t *testing.T) {
	db := seedPlanningFixture(t)
	awayCell := weather.GridCell{GridLat: -6.25, GridLng: 106.75}
	seedProjectionWeather(t, db, awayCell)

	if err := db.Create(&entity.Member{
		ID: "member-plot-away", CooperativeID: homeCoop,
		Name: "Anggota Jauh", CreatedAt: planningNow,
	}).Error; err != nil {
		t.Fatalf("seeding member: %v", err)
	}
	seedPlot(t, db, "plot-away", homeCoop, awayCell)

	useCase := planningUseCase(t, db)
	session, counter := countingSession(db)
	useCase.DB = session

	plots := []entity.Plot{}
	if err := db.Where("cooperative_id = ?", homeCoop).Find(&plots).Error; err != nil {
		t.Fatalf("reading plots: %v", err)
	}

	normals, err := useCase.normalsFor(context.Background(), plots)
	if err != nil {
		t.Fatalf("normalsFor: %v", err)
	}

	if len(normals) != 2 {
		t.Fatalf("len(normals) = %d, want 2 cells", len(normals))
	}
	if statements := counter.count.Load(); statements != 1 {
		t.Errorf("statements = %d, want 1 for every grid cell together", statements)
	}
}
