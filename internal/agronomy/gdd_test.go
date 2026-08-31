package agronomy_test

import (
	"testing"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

func day(date string, tmin, tmax float64) agronomy.TempDay {
	return agronomy.TempDay{Date: date, TMin: tmin, TMax: tmax}
}

func TestGddForDayIsMeanTemperatureMinusBase(t *testing.T) {
	if got := agronomy.GddForDay(day("2026-01-01", 20, 30), 10); got != 15 {
		t.Errorf("GddForDay = %v, want 15", got)
	}
}

func TestGddForDayNeverGoesNegative(t *testing.T) {
	if got := agronomy.GddForDay(day("2026-01-01", 2, 6), 10); got != 0 {
		t.Errorf("GddForDay = %v, want 0", got)
	}
}

func TestAccumulateGddIsMonotonicWithOneEntryPerDay(t *testing.T) {
	series := agronomy.AccumulateGdd([]agronomy.TempDay{
		day("2026-01-01", 20, 30),
		day("2026-01-02", 20, 30),
		day("2026-01-03", 5, 5),
	}, 10)

	want := []float64{15, 30, 30}
	if len(series) != len(want) {
		t.Fatalf("len(series) = %d, want %d", len(series), len(want))
	}
	for i, entry := range series {
		if entry.Gdd != want[i] {
			t.Errorf("series[%d].Gdd = %v, want %v", i, entry.Gdd, want[i])
		}
	}
}

func TestAccumulateGddOfNothingIsEmpty(t *testing.T) {
	if series := agronomy.AccumulateGdd(nil, 10); len(series) != 0 {
		t.Errorf("len(series) = %d, want 0", len(series))
	}
}

func TestGrowthStageForMapsFractionToStage(t *testing.T) {
	tests := []struct {
		accumulated float64
		want        constants.GrowthStage
	}{
		{0, constants.StageBare},
		{149, constants.StageBare},
		{150, constants.StageEstablished},
		{499, constants.StageEstablished},
		{500, constants.StageVegetative},
		{849, constants.StageVegetative},
		{850, constants.StageRipening},
		{999, constants.StageRipening},
		{1000, constants.StageReady},
		{1500, constants.StageReady},
	}
	for _, test := range tests {
		if got := agronomy.GrowthStageFor(test.accumulated, 1000); got != test.want {
			t.Errorf("GrowthStageFor(%v, 1000) = %d, want %d", test.accumulated, got, test.want)
		}
	}
}

func TestGrowthStageForZeroRequirementIsBare(t *testing.T) {
	if got := agronomy.GrowthStageFor(10, 0); got != constants.StageBare {
		t.Errorf("GrowthStageFor(10, 0) = %d, want %d", got, constants.StageBare)
	}
}
