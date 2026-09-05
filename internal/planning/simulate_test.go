package planning_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/planning"
)

func TestSimulatorWindowLandsInsideVarietyBounds(t *testing.T) {
	simulator := planning.NewSimulator(flatNormals(27.8))
	variety := lowlandRice()
	planting := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	window, plausibility, err := simulator.Window("variety-rice", variety, nil, planting)
	if err != nil {
		t.Fatalf("Window: %v", err)
	}

	if plausibility != constants.PlausibilityOk {
		t.Errorf("plausibility = %q, want %q", plausibility, constants.PlausibilityOk)
	}
	if !window.Start.After(planting) {
		t.Errorf("window starts %s, planted %s",
			agronomy.ToISODate(window.Start), agronomy.ToISODate(planting))
	}
	if !window.End.After(window.Start) {
		t.Error("window ends before it starts")
	}

	midDap := (agronomy.DaysBetween(planting, window.Start) +
		agronomy.DaysBetween(planting, window.End)) / 2
	if midDap < variety.DaysToHarvestMin || midDap > variety.DaysToHarvestMax {
		t.Errorf("mid DAP = %d, want within [%d, %d]",
			midDap, variety.DaysToHarvestMin, variety.DaysToHarvestMax)
	}
}

func TestSimulatorMemoisesRepeatedWindows(t *testing.T) {
	simulator := planning.NewSimulator(flatNormals(27.8))
	planting := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	for range 5 {
		if _, _, err := simulator.Window("variety-rice", lowlandRice(), nil, planting); err != nil {
			t.Fatalf("Window: %v", err)
		}
	}

	if simulator.Simulations() != 1 {
		t.Errorf("Simulations() = %d, want 1", simulator.Simulations())
	}
}

func TestSimulatorIsDeterministic(t *testing.T) {
	planting := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	first, _, err := planning.NewSimulator(flatNormals(27.8)).
		Window("variety-rice", lowlandRice(), nil, planting)
	if err != nil {
		t.Fatalf("Window: %v", err)
	}
	second, _, err := planning.NewSimulator(flatNormals(27.8)).
		Window("variety-rice", lowlandRice(), nil, planting)
	if err != nil {
		t.Fatalf("Window: %v", err)
	}

	if !first.Start.Equal(second.Start) || !first.End.Equal(second.End) {
		t.Errorf("windows differ: %v vs %v", first, second)
	}
}

func TestSimulatorYieldRangeUsesClimateNormalsNotBaseTemperature(t *testing.T) {
	simulator := planning.NewSimulator(flatNormals(27.8))
	variety := lowlandRice()
	planting := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	window, _, err := simulator.Window("variety-rice", variety, nil, planting)
	if err != nil {
		t.Fatalf("Window: %v", err)
	}

	observations := []agronomy.YieldObservation{}
	for i := range 12 {
		observations = append(observations, agronomy.YieldObservation{
			ActualYieldPerHa: 6.1,
			Features: agronomy.YieldFeatures{
				VarietyBaselineYieldPerHa: 6,
				GddRatio:                  1 + float64(i)*0.01,
				AreaHa:                    0.5,
				MeanTempC:                 27.8,
			},
		})
	}
	model := agronomy.FitYieldModel(observations)

	low, mid, high := simulator.YieldPerHaRange(model, variety, planting, window.End, 0.5)

	if !(low <= mid && mid <= high) {
		t.Fatalf("range out of order: %v %v %v", low, mid, high)
	}
	if mid < 4 || mid > 8 {
		t.Errorf("mid = %v, want a plausible rice yield between 4 and 8 t/ha", mid)
	}
}
