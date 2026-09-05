package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
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

func staggerUseCase(t *testing.T, db *gorm.DB) *StaggerUseCase {
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

func seedTightCapacity(t *testing.T, db *gorm.DB, tonnesPerWeek float64) {
	t.Helper()

	if err := db.Create(&entity.CooperativeCapacity{
		CooperativeID: homeCoop,
		CommodityID:   maizeCommodity,
		TonnesPerWeek: tonnesPerWeek,
	}).Error; err != nil {
		t.Fatalf("seeding capacity: %v", err)
	}
}

func seedFuturePlantings(t *testing.T, db *gorm.DB) {
	t.Helper()

	blocks := []entity.Block{
		{
			ID: "block-future-a", PlotID: "plot-home", Label: "BLOK C", AreaHa: 1, OrderIndex: 2,
			CommodityID: maizeCommodity, VarietyID: "variety-1",
			PlantingDate: agronomy.AddDays(projectionNow, 30),
		},
		{
			ID: "block-future-b", PlotID: "plot-home", Label: "BLOK D", AreaHa: 1, OrderIndex: 3,
			CommodityID: maizeCommodity, VarietyID: "variety-1",
			PlantingDate: agronomy.AddDays(projectionNow, 30),
		},
	}
	if err := db.Create(&blocks).Error; err != nil {
		t.Fatalf("seeding future blocks: %v", err)
	}
}

func suggestionNaming(t *testing.T, db *gorm.DB, blockID string) agronomy.StaggerSuggestion {
	t.Helper()

	loaded, err := dashboardUseCase(t, db).Load(context.Background(), homeCoop, constants.DefaultHorizonWeeks, projectionNow)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, suggestion := range loaded.Suggestions {
		if slices.Contains(suggestion.BlockIDs, blockID) {
			return suggestion
		}
	}

	t.Fatalf("no staggering suggestion names %s; suggestions were %+v",
		blockID, loaded.Suggestions)
	return agronomy.StaggerSuggestion{}
}

func pengurus() *entity.AppUser {
	cooperativeID := homeCoop
	return &entity.AppUser{
		ID: "pengurus-1", Role: constants.RolePengurus, CooperativeID: &cooperativeID,
	}
}

func TestApplyStaggerMovesThePlantingsAndLogsWhatMoved(t *testing.T) {
	db := dashboardFixture(t)
	seedTightCapacity(t, db, 1)
	seedFuturePlantings(t, db)

	suggestion := suggestionNaming(t, db, "block-future-a")

	applied, err := staggerUseCase(t, db).Apply(context.Background(), pengurus(),
		&model.ApplyStaggerRequest{
			ISOWeek: suggestion.ISOWeek, CommodityID: suggestion.CommodityID,
		}, projectionNow)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.Shifted == 0 {
		t.Fatal("Shifted = 0, want at least one planting moved")
	}

	moved := new(entity.Block)
	if err := db.Where("id = ?", "block-future-a").Take(moved).Error; err != nil {
		t.Fatalf("reading back the block: %v", err)
	}
	shift := agronomy.DaysBetween(agronomy.AddDays(projectionNow, 30), moved.PlantingDate)
	if shift != suggestion.ShiftDays {
		t.Errorf("planting moved %d days, want the suggestion's %d", shift, suggestion.ShiftDays)
	}
}

func TestApplyStaggerWritesTheDatesAndTheLogAsOneEvent(t *testing.T) {
	db := dashboardFixture(t)
	seedTightCapacity(t, db, 1)
	seedFuturePlantings(t, db)

	suggestion := suggestionNaming(t, db, "block-future-a")

	applied, err := staggerUseCase(t, db).Apply(context.Background(), pengurus(),
		&model.ApplyStaggerRequest{
			ISOWeek: suggestion.ISOWeek, CommodityID: suggestion.CommodityID,
		}, projectionNow)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	cooperative := new(entity.Cooperative)
	if err := db.Where("id = ?", homeCoop).Take(cooperative).Error; err != nil {
		t.Fatalf("reading back the cooperative: %v", err)
	}

	entries := []staggerLogEntry{}
	if err := json.Unmarshal(cooperative.StaggerApplied, &entries); err != nil {
		t.Fatalf("reading the stagger log: %v", err)
	}
	if len(entries) != applied.Shifted {
		t.Fatalf("log has %d entries but %d plantings moved: a log entry without a date change invents a diversion",
			len(entries), applied.Shifted)
	}

	for _, entry := range entries {
		original, err := agronomy.UTCDate(entry.OriginalDate)
		if err != nil {
			t.Fatalf("log entry %+v carries a malformed original date", entry)
		}
		shifted, err := agronomy.UTCDate(entry.ShiftedDate)
		if err != nil {
			t.Fatalf("log entry %+v carries a malformed shifted date", entry)
		}

		block := new(entity.Block)
		if err := db.Where("id = ?", entry.BlockID).Take(block).Error; err != nil {
			t.Fatalf("log names block %s, which does not exist", entry.BlockID)
		}
		if !block.PlantingDate.Equal(shifted) {
			t.Errorf("block %s is planted %v, but the log says %v",
				entry.BlockID, block.PlantingDate, shifted)
		}
		if agronomy.DaysBetween(original, shifted) != suggestion.ShiftDays {
			t.Errorf("log entry %+v records a shift the suggestion did not propose", entry)
		}
	}
}

func TestApplyStaggerAppendsToAnExistingLog(t *testing.T) {
	db := dashboardFixture(t)
	seedTightCapacity(t, db, 1)
	seedFuturePlantings(t, db)

	existing := json.RawMessage(`[{"season_label":"MT-2025","block_id":"older",
		"original_date":"2025-09-07","shifted_date":"2025-09-14"}]`)
	if err := db.Model(&entity.Cooperative{}).Where("id = ?", homeCoop).
		Update("stagger_applied", existing).Error; err != nil {
		t.Fatalf("seeding an existing log: %v", err)
	}

	suggestion := suggestionNaming(t, db, "block-future-a")
	applied, err := staggerUseCase(t, db).Apply(context.Background(), pengurus(),
		&model.ApplyStaggerRequest{
			ISOWeek: suggestion.ISOWeek, CommodityID: suggestion.CommodityID,
		}, projectionNow)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	cooperative := new(entity.Cooperative)
	if err := db.Where("id = ?", homeCoop).Take(cooperative).Error; err != nil {
		t.Fatalf("reading back the cooperative: %v", err)
	}
	entries := []staggerLogEntry{}
	if err := json.Unmarshal(cooperative.StaggerApplied, &entries); err != nil {
		t.Fatalf("reading the stagger log: %v", err)
	}

	if len(entries) != applied.Shifted+1 {
		t.Errorf("log has %d entries, want the earlier one kept: figure 4 reads all of them",
			len(entries))
	}
	if entries[0].BlockID != "older" {
		t.Errorf("entries[0] = %+v, want the earlier entry first", entries[0])
	}
}

func TestApplyStaggerRefusesASuggestionThatNoLongerHolds(t *testing.T) {
	db := dashboardFixture(t)
	seedTightCapacity(t, db, 1)
	seedFuturePlantings(t, db)

	_, err := staggerUseCase(t, db).Apply(context.Background(), pengurus(),
		&model.ApplyStaggerRequest{
			ISOWeek: "2020-W01", CommodityID: maizeCommodity,
		}, projectionNow)

	if !errors.Is(err, ErrSuggestionStale) {
		t.Errorf("err = %v, want ErrSuggestionStale", err)
	}
}

func TestApplyStaggerRefusesAMalformedWeek(t *testing.T) {
	db := dashboardFixture(t)
	seedTightCapacity(t, db, 1)
	seedFuturePlantings(t, db)

	_, err := staggerUseCase(t, db).Apply(context.Background(), pengurus(),
		&model.ApplyStaggerRequest{
			ISOWeek: "minggu depan", CommodityID: maizeCommodity,
		}, projectionNow)

	if !errors.Is(err, ErrSuggestionStale) {
		t.Errorf("err = %v, want ErrSuggestionStale", err)
	}
}

func TestApplyStaggerRefusesWhenEveryBlockIsAlreadyPlanted(t *testing.T) {
	db := dashboardFixture(t)
	seedTightCapacity(t, db, 0.001)

	suggestion := suggestionNaming(t, db, "block-growing")

	_, err := staggerUseCase(t, db).Apply(context.Background(), pengurus(),
		&model.ApplyStaggerRequest{
			ISOWeek: suggestion.ISOWeek, CommodityID: suggestion.CommodityID,
		}, projectionNow)

	var refusal *StaggerRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a *StaggerRefusal", err)
	}
	if refusal.Code != constants.StaggerNothingToShift {
		t.Errorf("Code = %q, want %q", refusal.Code, constants.StaggerNothingToShift)
	}
	if refusal.AlreadyPlanted == 0 {
		t.Error("AlreadyPlanted = 0, want the count the screen explains the refusal with")
	}

	unchanged := new(entity.Block)
	if err := db.Where("id = ?", "block-growing").Take(unchanged).Error; err != nil {
		t.Fatalf("reading back the block: %v", err)
	}
	if !unchanged.PlantingDate.Equal(agronomy.AddDays(projectionNow, -60)) {
		t.Errorf("PlantingDate = %v, want it untouched", unchanged.PlantingDate)
	}
}

func TestApplyStaggerRefusesAnAccountWithNoCooperative(t *testing.T) {
	db := dashboardFixture(t)
	buyer := &entity.AppUser{ID: "buyer-1", Role: constants.RoleBuyer}

	_, err := staggerUseCase(t, db).Apply(context.Background(), buyer,
		&model.ApplyStaggerRequest{
			ISOWeek: "2026-W40", CommodityID: maizeCommodity,
		}, projectionNow)

	if !errors.Is(err, ErrNoCooperative) {
		t.Errorf("err = %v, want ErrNoCooperative", err)
	}
}
