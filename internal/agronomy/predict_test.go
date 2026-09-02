package agronomy_test

import (
	"math"
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

var (
	planted = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	maize = agronomy.Variety{
		GddRequirement: 1400, BaseTempC: 10,
		DaysToHarvestMin: 90, DaysToHarvestMax: 110,
		YieldPerHaMin: 7, YieldPerHaMax: 9.5,
	}

	rice = agronomy.Variety{
		GddRequirement: 1950, BaseTempC: 10,
		DaysToHarvestMin: 105, DaysToHarvestMax: 125,
		YieldPerHaMin: 5, YieldPerHaMax: 7,
	}
)

func flatNormals(meanC, sdC float64) []agronomy.ClimateNormal {
	normals := make([]agronomy.ClimateNormal, 366)
	for i := range normals {
		normals[i] = agronomy.ClimateNormal{DayOfYear: i + 1, MeanC: meanC, SdC: sdC}
	}
	return normals
}

func observedDays(from time.Time, count int, meanC float64) []agronomy.TempDay {
	days := make([]agronomy.TempDay, count)
	for i := range days {
		days[i] = agronomy.TempDay{
			Date: agronomy.ToISODate(agronomy.AddDays(from, i)),
			TMin: meanC - 4,
			TMax: meanC + 4,
		}
	}
	return days
}

func closeTo(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %v, want %v (tolerance %v)", name, got, want, tolerance)
	}
}

func predictOrFail(t *testing.T, input agronomy.HarvestInput) agronomy.HarvestWindow {
	t.Helper()
	window, err := agronomy.PredictHarvest(input)
	if err != nil {
		t.Fatalf("PredictHarvest: %v", err)
	}
	return window
}

func TestPredictHarvestReturnsWindowNeverPoint(t *testing.T) {
	window := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted,
		Climatology:  flatNormals(26, 2),
		Variety:      maize,
	})

	if !window.End.After(window.Start) {
		t.Errorf("End %v is not after Start %v", window.End, window.Start)
	}
	if window.Confidence != constants.HarvestWindowConfidence {
		t.Errorf("Confidence = %v, want %v", window.Confidence, constants.HarvestWindowConfidence)
	}
}

func TestPredictHarvestReportsThermalWindowUntrimmedBelowDayFloor(t *testing.T) {
	window := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted,
		Climatology:  flatNormals(27, 1),
		Variety:      maize,
	})

	if days := agronomy.DaysBetween(planted, window.Start); days >= maize.DaysToHarvestMin {
		t.Errorf("window starts %d days after planting, want fewer than %d",
			days, maize.DaysToHarvestMin)
	}
}

func TestPredictHarvestNarrowsAsObservedWeatherReplacesClimatology(t *testing.T) {
	normals := flatNormals(26, 2)

	early := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Observed: observedDays(planted, 10, 26),
		Climatology: normals, Variety: maize,
	})
	late := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Observed: observedDays(planted, 80, 26),
		Climatology: normals, Variety: maize,
	})

	earlyWidth := agronomy.DaysBetween(early.Start, early.End)
	lateWidth := agronomy.DaysBetween(late.Start, late.End)
	if lateWidth > earlyWidth {
		t.Errorf("width with 80 observed days = %d, want at most %d", lateWidth, earlyWidth)
	}
}

func TestPredictHarvestReportsAccumulatedGddAndSeries(t *testing.T) {
	window := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Observed: observedDays(planted, 10, 26),
		Climatology: flatNormals(26, 2), Variety: maize,
	})

	closeTo(t, "GddAccumulated", window.GddAccumulated, 160, 0.5)
	if window.GddRequired != 1400 {
		t.Errorf("GddRequired = %v, want 1400", window.GddRequired)
	}
	if len(window.CumulativeGdd) < 10 {
		t.Errorf("len(CumulativeGdd) = %d, want at least 10", len(window.CumulativeGdd))
	}
}

func TestPredictHarvestProjectsGddPastTheLastKnownDayUntilMaturity(t *testing.T) {
	window := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Observed: observedDays(planted, 10, 26),
		Climatology: flatNormals(26, 2), Variety: maize,
	})

	if window.ProjectedFrom == nil {
		t.Fatal("ProjectedFrom = nil, want the day after the last observed reading")
	}
	if !window.ProjectedFrom.Equal(agronomy.AddDays(planted, 10)) {
		t.Errorf("ProjectedFrom = %v, want %v", *window.ProjectedFrom, agronomy.AddDays(planted, 10))
	}

	last := window.CumulativeGdd[len(window.CumulativeGdd)-1]
	if last.Gdd < maize.GddRequirement {
		t.Errorf("final projected Gdd = %v, want it to reach the %v requirement",
			last.Gdd, maize.GddRequirement)
	}
	// Ascending and gapless: the slider looks up a day by scanning this series,
	// so a hole or a step backwards would make a date resolve to the wrong stage.
	for i := 1; i < len(window.CumulativeGdd); i++ {
		if window.CumulativeGdd[i].Gdd < window.CumulativeGdd[i-1].Gdd {
			t.Fatalf("CumulativeGdd decreases at day %d: %v then %v",
				i, window.CumulativeGdd[i-1].Gdd, window.CumulativeGdd[i].Gdd)
		}
	}
}

func TestPredictHarvestLeavesProjectedFromNilOnceAlreadyMature(t *testing.T) {
	// 16 Gdd/day (mean 26, base 10) x 90 days = 1440, past maize's 1400 requirement.
	window := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Observed: observedDays(planted, 90, 26),
		Climatology: flatNormals(26, 2), Variety: maize,
	})

	if window.ProjectedFrom != nil {
		t.Errorf("ProjectedFrom = %v, want nil once GddAccumulated already clears the requirement",
			*window.ProjectedFrom)
	}
}

func TestPredictHarvestBasis(t *testing.T) {
	normals := flatNormals(26, 2)

	observed := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Observed: observedDays(planted, 10, 26),
		Climatology: normals, Variety: maize,
	})
	if observed.Basis != constants.BasisObserved {
		t.Errorf("Basis = %q, want %q", observed.Basis, constants.BasisObserved)
	}

	forecast := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Forecast: observedDays(planted, 10, 26),
		Climatology: normals, Variety: maize,
	})
	if forecast.Basis != constants.BasisForecast {
		t.Errorf("Basis = %q, want %q", forecast.Basis, constants.BasisForecast)
	}

	bare := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Climatology: normals, Variety: maize,
	})
	if bare.Basis != constants.BasisClimatology {
		t.Errorf("Basis = %q, want %q", bare.Basis, constants.BasisClimatology)
	}
}

func TestPredictHarvestDoesNotDoubleCountOverlappingDays(t *testing.T) {
	overlap := observedDays(planted, 10, 26)

	window := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Observed: overlap, Forecast: overlap,
		Climatology: flatNormals(26, 2), Variety: maize,
	})

	closeTo(t, "GddAccumulated", window.GddAccumulated, 160, 0.5)
	// Not exactly 10: maize is nowhere near its GDD requirement at day 10, so
	// the series keeps going past the known days as a climatology projection.
	// What overlapping Observed/Forecast must not do is inflate those first
	// ten -- which GddAccumulated above already confirms.
	if len(window.CumulativeGdd) < 10 {
		t.Errorf("len(CumulativeGdd) = %d, want at least 10", len(window.CumulativeGdd))
	}
}

func TestPredictHarvestShiftsLaterUnderPositiveCalibration(t *testing.T) {
	normals := flatNormals(26, 2)

	base := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Climatology: normals, Variety: maize,
	})
	shifted := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Climatology: normals, Variety: maize,
		Calibration: &agronomy.Calibration{OffsetDays: 8, NObservations: 30},
	})

	if shifted.End.Before(base.End) {
		t.Errorf("calibrated End %v is before uncalibrated %v", shifted.End, base.End)
	}
}

func TestPredictHarvestShrinksCalibrationWithFewObservations(t *testing.T) {
	normals := flatNormals(26, 2)

	one := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Climatology: normals, Variety: maize,
		Calibration: &agronomy.Calibration{OffsetDays: 8, NObservations: 1},
	})
	many := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Climatology: normals, Variety: maize,
		Calibration: &agronomy.Calibration{OffsetDays: 8, NObservations: 100},
	})

	if !one.End.Before(many.End) {
		t.Errorf("End with 1 observation %v, want earlier than with 100 %v", one.End, many.End)
	}
}

func TestPredictHarvestAlwaysYieldsStartBeforeEnd(t *testing.T) {
	for _, days := range []int{0, 5, 40, 88, 120} {
		window := predictOrFail(t, agronomy.HarvestInput{
			PlantingDate: planted, Observed: observedDays(planted, days, 26),
			Climatology: flatNormals(26, 2), Variety: maize,
		})
		if window.Start.After(window.End) {
			t.Errorf("with %d observed days Start %v is after End %v",
				days, window.Start, window.End)
		}
	}
}

func TestPredictHarvestPlausibility(t *testing.T) {
	tests := []struct {
		name    string
		normals []agronomy.ClimateNormal
		variety agronomy.Variety
		want    constants.Plausibility
	}{
		{"thermal window agrees with day bounds", flatNormals(27, 1), rice, constants.PlausibilityOk},
		{"matures sooner than the catalogue", flatNormals(27, 1), maize, constants.PlausibilityEarly},
		{"matures later than the catalogue", flatNormals(24, 1), rice, constants.PlausibilityLate},
		{"runs far past the day ceiling", flatNormals(22, 1), rice, constants.PlausibilityImplausible},
		{"runs far under the day floor", flatNormals(33, 1), maize, constants.PlausibilityImplausible},
		{"never reaches its requirement", flatNormals(5, 1), maize, constants.PlausibilityImplausible},
	}

	for _, test := range tests {
		window := predictOrFail(t, agronomy.HarvestInput{
			PlantingDate: planted, Climatology: test.normals, Variety: test.variety,
		})

		if window.Plausibility != test.want {
			t.Errorf("%s: Plausibility = %q, want %q", test.name, window.Plausibility, test.want)
		}
		wantImplausible := test.want == constants.PlausibilityImplausible
		if agronomy.IsImplausible(window) != wantImplausible {
			t.Errorf("%s: IsImplausible = %v, want %v",
				test.name, agronomy.IsImplausible(window), wantImplausible)
		}
	}
}

func TestPredictHarvestStaysARangeWhenBoundsDisagree(t *testing.T) {
	window := predictOrFail(t, agronomy.HarvestInput{
		PlantingDate: planted, Climatology: flatNormals(22, 1), Variety: rice,
	})

	if !window.End.After(window.Start) {
		t.Errorf("End %v is not after Start %v", window.End, window.Start)
	}
}
