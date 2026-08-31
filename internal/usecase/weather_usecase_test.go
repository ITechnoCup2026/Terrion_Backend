package usecase

import (
	"testing"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/weather"
)

var testCell = weather.GridCell{GridLat: -6.25, GridLng: 107.75}

func TestDailyRowsForKeepsTheLastReadingOfADay(t *testing.T) {
	forecast := agronomy.TempDay{Date: "2026-03-02", TMin: 20, TMax: 30}
	observed := agronomy.TempDay{Date: "2026-03-02", TMin: 22, TMax: 33}

	rows, err := dailyRowsFor(testCell, []agronomy.TempDay{forecast, observed})
	if err != nil {
		t.Fatalf("dailyRowsFor: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1: a day may reach the upsert only once", len(rows))
	}
	if rows[0].TempMin != 22 || rows[0].TempMax != 33 {
		t.Errorf("rows[0] = %+v, want the observed reading 22/33", rows[0])
	}
}

func TestDailyRowsForKeepsDistinctDaysInOrder(t *testing.T) {
	rows, err := dailyRowsFor(testCell, []agronomy.TempDay{
		{Date: "2026-03-02", TMin: 20, TMax: 30},
		{Date: "2026-03-03", TMin: 21, TMax: 31},
		{Date: "2026-03-02", TMin: 22, TMax: 33},
	})
	if err != nil {
		t.Fatalf("dailyRowsFor: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if agronomy.ToISODate(rows[0].Date) != "2026-03-02" || rows[0].TempMin != 22 {
		t.Errorf("rows[0] = %+v, want the overwritten 2026-03-02", rows[0])
	}
	if agronomy.ToISODate(rows[1].Date) != "2026-03-03" {
		t.Errorf("rows[1] = %+v, want 2026-03-03", rows[1])
	}
}

func TestDailyRowsForStampsTheCellOntoEveryRow(t *testing.T) {
	rows, err := dailyRowsFor(testCell, []agronomy.TempDay{{Date: "2026-03-02", TMin: 20, TMax: 30}})
	if err != nil {
		t.Fatalf("dailyRowsFor: %v", err)
	}

	if rows[0].GridLat != testCell.GridLat || rows[0].GridLng != testCell.GridLng {
		t.Errorf("rows[0] cell = %v/%v, want %v/%v",
			rows[0].GridLat, rows[0].GridLng, testCell.GridLat, testCell.GridLng)
	}
}

func TestDailyRowsForRejectsAMalformedDate(t *testing.T) {
	if _, err := dailyRowsFor(testCell, []agronomy.TempDay{{Date: "02-03-2026"}}); err == nil {
		t.Error("dailyRowsFor returned nil error, want a parse failure")
	}
}
