package usecase

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

var (
	projectionNow = time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	homeCoop      = "11111111-1111-4111-8111-111111111111"
	otherCoop     = "22222222-2222-4222-8222-222222222222"
	homeCell      = weather.GridCell{GridLat: -6.25, GridLng: 107.75}
)

func projectionDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&entity.Plot{}, &entity.Block{}, &entity.Variety{}, &entity.Calibration{},
		&entity.WeatherDaily{}, &entity.WeatherNormal{},
	); err != nil {
		t.Fatalf("migrating projection tables: %v", err)
	}
	return db
}

func seedPlot(t *testing.T, db *gorm.DB, id, cooperativeID string, cell weather.GridCell) {
	t.Helper()

	err := db.Exec(`INSERT INTO plot
		(id, cooperative_id, member_id, public_id, name, area_ha, lat, lng,
		 grid_lat, grid_lng, tile_size_m2, terrain_seed, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, cooperativeID, "member-"+id, "pub-"+id, "Lahan "+id, 1.0,
		cell.GridLat, cell.GridLng, cell.GridLat, cell.GridLng, 100, 1, projectionNow).Error
	if err != nil {
		t.Fatalf("seeding plot %s: %v", id, err)
	}
}

func seedProjectionWeather(t *testing.T, db *gorm.DB, cell weather.GridCell) {
	t.Helper()

	daily := []entity.WeatherDaily{}
	for offset := -400; offset <= 16; offset++ {
		daily = append(daily, entity.WeatherDaily{
			GridLat: cell.GridLat,
			GridLng: cell.GridLng,
			Date:    agronomy.AddDays(projectionNow, offset),
			TempMin: 22,
			TempMax: 30,
		})
	}
	if err := db.CreateInBatches(daily, 200).Error; err != nil {
		t.Fatalf("seeding weather_daily: %v", err)
	}

	normals := make([]entity.WeatherNormal, 366)
	for i := range normals {
		normals[i] = entity.WeatherNormal{
			GridLat: cell.GridLat, GridLng: cell.GridLng,
			DayOfYear: i + 1, MeanC: 26, SdC: 2,
		}
	}
	if err := db.CreateInBatches(normals, 200).Error; err != nil {
		t.Fatalf("seeding weather_normals: %v", err)
	}
}

func projectionUseCase(t *testing.T, db *gorm.DB) *ProjectionUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	weatherUseCase := NewWeatherUseCase(db, log, &repository.WeatherRepository{}, nil)

	return NewProjectionUseCase(db, log,
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.CalibrationRepository{},
		weatherUseCase)
}

func seedProjectionFixture(t *testing.T) *gorm.DB {
	t.Helper()

	db := projectionDB(t)
	seedProjectionWeather(t, db, homeCell)
	seedPlot(t, db, "plot-home", homeCoop, homeCell)
	seedPlot(t, db, "plot-other", otherCoop, homeCell)

	maize := &entity.Variety{
		ID: "variety-1", CommodityID: "commodity-1", Name: "Bisi-18",
		GddRequirement: 1400, BaseTempC: 10,
		DaysToHarvestMin: 90, DaysToHarvestMax: 110,
		YieldPerHaMin: 7, YieldPerHaMax: 9.5,
	}
	if err := db.Create(maize).Error; err != nil {
		t.Fatalf("seeding variety: %v", err)
	}

	harvestDate := agronomy.AddDays(projectionNow, -100)
	yieldKg := 8000.0

	blocks := []entity.Block{
		{
			ID: "block-growing", PlotID: "plot-home", Label: "BLOK A", AreaHa: 1,
			CommodityID: "commodity-1", VarietyID: "variety-1",
			PlantingDate: agronomy.AddDays(projectionNow, -60),
		},
		{
			ID: "block-harvested", PlotID: "plot-home", Label: "BLOK B", AreaHa: 1, OrderIndex: 1,
			CommodityID: "commodity-1", VarietyID: "variety-1",
			PlantingDate:      agronomy.AddDays(projectionNow, -200),
			ActualHarvestDate: &harvestDate,
			ActualYieldKg:     &yieldKg,
		},
		{
			ID: "block-other-coop", PlotID: "plot-other", Label: "BLOK A", AreaHa: 1,
			CommodityID: "commodity-1", VarietyID: "variety-1",
			PlantingDate: agronomy.AddDays(projectionNow, -60),
		},
	}
	if err := db.Create(&blocks).Error; err != nil {
		t.Fatalf("seeding blocks: %v", err)
	}

	return db
}

func TestProjectCooperativeProjectsOnlyGrowingBlocks(t *testing.T) {
	db := seedProjectionFixture(t)

	result, err := projectionUseCase(t, db).
		ProjectCooperative(context.Background(), homeCoop, projectionNow)
	if err != nil {
		t.Fatalf("ProjectCooperative: %v", err)
	}

	if len(result.Projections) != 1 {
		t.Fatalf("len(Projections) = %d, want 1: only the unharvested block is projected",
			len(result.Projections))
	}
	if result.Projections[0].BlockID != "block-growing" {
		t.Errorf("BlockID = %q, want block-growing", result.Projections[0].BlockID)
	}
}

func TestProjectCooperativeStaysInsideItsCooperative(t *testing.T) {
	db := seedProjectionFixture(t)

	result, err := projectionUseCase(t, db).
		ProjectCooperative(context.Background(), homeCoop, projectionNow)
	if err != nil {
		t.Fatalf("ProjectCooperative: %v", err)
	}

	if len(result.Plots) != 1 || result.Plots[0].ID != "plot-home" {
		t.Fatalf("Plots = %+v, want only plot-home", result.Plots)
	}
	for _, projection := range result.Projections {
		if projection.PlotID == "plot-other" {
			t.Errorf("projected block %s belongs to another cooperative", projection.BlockID)
		}
	}
}

func TestProjectCooperativeReturnsAWindowAndTonnage(t *testing.T) {
	db := seedProjectionFixture(t)

	result, err := projectionUseCase(t, db).
		ProjectCooperative(context.Background(), homeCoop, projectionNow)
	if err != nil {
		t.Fatalf("ProjectCooperative: %v", err)
	}

	projection := result.Projections[0]
	if !projection.Window.End.After(projection.Window.Start) {
		t.Errorf("window %v..%v is not a range", projection.Window.Start, projection.Window.End)
	}
	if projection.ExpectedTonnes <= 0 {
		t.Errorf("ExpectedTonnes = %v, want a positive tonnage", projection.ExpectedTonnes)
	}

	window, projected := result.Windows[projection.BlockID]
	if !projected {
		t.Fatalf("Windows has no entry for %s", projection.BlockID)
	}
	if window.GddRequired != 1400 {
		t.Errorf("GddRequired = %v, want the variety's 1400", window.GddRequired)
	}
	if window.GddAccumulated <= 0 {
		t.Errorf("GddAccumulated = %v, want heat from 60 days of stored weather",
			window.GddAccumulated)
	}
}

func TestProjectCooperativeWithoutPlotsIsEmpty(t *testing.T) {
	db := seedProjectionFixture(t)

	result, err := projectionUseCase(t, db).ProjectCooperative(
		context.Background(), "33333333-3333-4333-8333-333333333333", projectionNow)
	if err != nil {
		t.Fatalf("ProjectCooperative: %v", err)
	}

	if len(result.Plots) != 0 || len(result.Projections) != 0 || len(result.Windows) != 0 {
		t.Errorf("result = %+v, want everything empty", result)
	}
}

func TestProjectCooperativeMarksABlockWithNoStoredWeatherImplausible(t *testing.T) {
	db := seedProjectionFixture(t)
	seedPlot(t, db, "plot-dry", homeCoop, weather.GridCell{GridLat: 1.5, GridLng: 99.75})

	block := &entity.Block{
		ID: "block-dry", PlotID: "plot-dry", Label: "BLOK A", AreaHa: 1,
		CommodityID: "commodity-1", VarietyID: "variety-1",
		PlantingDate: agronomy.AddDays(projectionNow, -60),
	}
	if err := db.Create(block).Error; err != nil {
		t.Fatalf("seeding block: %v", err)
	}

	result, err := projectionUseCase(t, db).
		ProjectCooperative(context.Background(), homeCoop, projectionNow)
	if err != nil {
		t.Fatalf("ProjectCooperative: %v", err)
	}

	window, projected := result.Windows["block-dry"]
	if !projected {
		t.Fatal("block-dry was dropped; a plot with no weather must still appear, marked")
	}
	if window.Basis != constants.BasisClimatology {
		t.Errorf("Basis = %q, want %q", window.Basis, constants.BasisClimatology)
	}
	if !agronomy.IsImplausible(window) {
		t.Errorf("Plausibility = %q, want implausible: no weather cannot yield a confident date",
			window.Plausibility)
	}

	dated, projectedWithWeather := result.Windows["block-growing"]
	if !projectedWithWeather || agronomy.IsImplausible(dated) {
		t.Errorf("block-growing plausibility = %q, want a usable window", dated.Plausibility)
	}
}
