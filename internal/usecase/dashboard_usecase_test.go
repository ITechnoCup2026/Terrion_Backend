package usecase

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
)

func dashboardUseCase(t *testing.T, db *gorm.DB) *DashboardUseCase {
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

func dashboardFixture(t *testing.T) *gorm.DB {
	t.Helper()

	db, _ := plotFixture(t)
	if err := db.AutoMigrate(
		&entity.Cooperative{}, &entity.CooperativeCapacity{},
		&entity.ReferencePrice{}, &entity.InputOrder{}, &entity.InputOrderLine{},
	); err != nil {
		t.Fatalf("migrating dashboard tables: %v", err)
	}

	if err := db.Create(&entity.Cooperative{
		ID: homeCoop, Name: "KUD Subang", Village: "Jalancagak",
		District: "Subang", Province: "Jawa Barat", Lat: -6.25, Lng: 107.75,
	}).Error; err != nil {
		t.Fatalf("seeding cooperative: %v", err)
	}

	return db
}

func TestDashboardLoadReturnsAFullHorizonAndUpcomingRows(t *testing.T) {
	db := dashboardFixture(t)

	loaded, err := dashboardUseCase(t, db).Load(context.Background(), homeCoop, projectionNow)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Weeks) != constants.DefaultHorizonWeeks {
		t.Errorf("len(Weeks) = %d, want %d", len(loaded.Weeks), constants.DefaultHorizonWeeks)
	}
	if loaded.Commodities[maizeCommodity] != "Jagung" {
		t.Errorf("Commodities = %v, want the maize commodity named Jagung", loaded.Commodities)
	}
}

func TestDashboardLoadReportsNoImpactBeforeAnythingHasHappened(t *testing.T) {
	db := dashboardFixture(t)

	loaded, err := dashboardUseCase(t, db).Load(context.Background(), homeCoop, projectionNow)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Impact.InputCostSaved != nil {
		t.Errorf("InputCostSaved = %v, want nil: no order has completed",
			*loaded.Impact.InputCostSaved)
	}
	if loaded.Impact.TonnesDiverted != nil {
		t.Errorf("TonnesDiverted = %v, want nil: no staggering has been accepted",
			*loaded.Impact.TonnesDiverted)
	}
	if loaded.Impact.DaysToPayment != nil {
		t.Errorf("DaysToPayment = %v, want nil: nothing has been paid",
			*loaded.Impact.DaysToPayment)
	}
}

func TestDashboardLoadStaysInsideItsCooperative(t *testing.T) {
	db := dashboardFixture(t)

	loaded, err := dashboardUseCase(t, db).Load(context.Background(), homeCoop, projectionNow)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, row := range loaded.Upcoming {
		if row.PlotID == "plot-other" {
			t.Errorf("upcoming row %s belongs to another cooperative", row.BlockID)
		}
	}
}

func TestParseStaggerLogKeepsWellFormedEntries(t *testing.T) {
	raw := json.RawMessage(`[
		{"season_label":"MT-2026","block_id":"b1",
		 "original_date":"2026-09-07","shifted_date":"2026-09-14"}
	]`)

	records := parseStaggerLog(raw)

	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].BlockID != "b1" || records[0].SeasonLabel != "MT-2026" {
		t.Errorf("record = %+v, want b1 / MT-2026", records[0])
	}
	if agronomy.DaysBetween(records[0].OriginalDate, records[0].ShiftedDate) != 7 {
		t.Errorf("shift = %d days, want 7",
			agronomy.DaysBetween(records[0].OriginalDate, records[0].ShiftedDate))
	}
}

func TestParseStaggerLogDropsMalformedEntriesRatherThanFailing(t *testing.T) {
	raw := json.RawMessage(`[
		{"block_id":"","original_date":"2026-09-07","shifted_date":"2026-09-14"},
		{"block_id":"b2","original_date":"07-09-2026","shifted_date":"2026-09-14"},
		{"block_id":"b3","original_date":"2026-09-07"},
		{"block_id":"good","original_date":"2026-09-07","shifted_date":"2026-09-14"}
	]`)

	records := parseStaggerLog(raw)

	if len(records) != 1 || records[0].BlockID != "good" {
		t.Errorf("records = %+v, want only the well-formed entry: one bad row must not take the whole dashboard down", records)
	}
}

func TestParseStaggerLogOfAnEmptyOrBrokenColumnIsNothing(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`[]`), json.RawMessage(`{"not":"an array"}`)} {
		if records := parseStaggerLog(raw); len(records) != 0 {
			t.Errorf("parseStaggerLog(%s) = %+v, want nothing", raw, records)
		}
	}
}
