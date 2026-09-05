package agronomy_test

import (
	"math"
	"testing"

	"terrion-backend/internal/agronomy"
)

const (
	baselineYield  = 5.0
	trueHeatEffect = 0.5
	yieldPrecision = 0.05
	exactPrecision = 5e-6
)

func syntheticHistory(count int) []agronomy.YieldObservation {
	history := make([]agronomy.YieldObservation, count)
	for i := range history {
		gddRatio := 0.8 + float64((i*7)%41)/100
		index := 1 + trueHeatEffect*(gddRatio-1)

		history[i] = agronomy.YieldObservation{
			ActualYieldPerHa: baselineYield * index,
			Features: agronomy.YieldFeatures{
				VarietyBaselineYieldPerHa: baselineYield,
				GddRatio:                  gddRatio,
				AreaHa:                    0.3 + float64((i*13)%20)/10,
				MeanTempC:                 24 + float64((i*11)%60)/10,
			},
		}
	}
	return history
}

func featuresAt(gddRatio, baseline float64) agronomy.YieldFeatures {
	return agronomy.YieldFeatures{
		VarietyBaselineYieldPerHa: baseline,
		GddRatio:                  gddRatio,
		AreaHa:                    1.0,
		MeanTempC:                 27,
	}
}

func TestFitYieldModelFallsBackToCatalogueWithoutHistory(t *testing.T) {
	model := agronomy.FitYieldModel(nil)

	if model.NObservations != 0 {
		t.Errorf("NObservations = %d, want 0", model.NObservations)
	}
	closeTo(t, "PredictYieldPerHa",
		agronomy.PredictYieldPerHa(model, featuresAt(1.0, baselineYield)),
		baselineYield, exactPrecision)
}

func TestFitYieldModelRecoversInjectedRelationship(t *testing.T) {
	model := agronomy.FitYieldModel(syntheticHistory(100))

	closeTo(t, "yield at gddRatio 1.2",
		agronomy.PredictYieldPerHa(model, featuresAt(1.2, baselineYield)),
		baselineYield*1.1, yieldPrecision)
	closeTo(t, "yield at gddRatio 0.8",
		agronomy.PredictYieldPerHa(model, featuresAt(0.8, baselineYield)),
		baselineYield*0.9, yieldPrecision)
}

func TestFitYieldModelShrinksTowardCatalogueWithFewHarvests(t *testing.T) {
	many := agronomy.FitYieldModel(syntheticHistory(100))
	few := agronomy.FitYieldModel(syntheticHistory(100)[:6])

	distance := func(model agronomy.YieldModel) float64 {
		return math.Abs(agronomy.PredictYieldPerHa(model, featuresAt(1.2, baselineYield)) - baselineYield)
	}

	if distance(few) >= distance(many) {
		t.Errorf("distance with 6 harvests = %v, want less than with 100 = %v",
			distance(few), distance(many))
	}
}

func TestFitYieldModelReportsNearZeroResidualOnCleanData(t *testing.T) {
	if got := agronomy.FitYieldModel(syntheticHistory(100)).ResidualSd; got >= 0.05 {
		t.Errorf("ResidualSd = %v, want below 0.05", got)
	}
}

func TestFitYieldModelSurvivesAFeatureThatNeverVaries(t *testing.T) {
	history := syntheticHistory(40)
	for i := range history {
		history[i].Features.AreaHa = 1.0
	}

	got := agronomy.PredictYieldPerHa(agronomy.FitYieldModel(history), featuresAt(1.1, baselineYield))
	if math.IsInf(got, 0) || math.IsNaN(got) {
		t.Errorf("PredictYieldPerHa = %v, want a finite number", got)
	}
}

func TestPredictYieldPerHaScalesByVarietyBaseline(t *testing.T) {
	model := agronomy.FitYieldModel(syntheticHistory(100))

	riceYield := agronomy.PredictYieldPerHa(model, featuresAt(1.1, 5))
	potatoYield := agronomy.PredictYieldPerHa(model, featuresAt(1.1, 20))

	closeTo(t, "potato / rice", potatoYield/riceYield, 4, exactPrecision)
}

func TestPredictYieldPerHaNeverGoesNegative(t *testing.T) {
	model := agronomy.FitYieldModel(syntheticHistory(100))

	if got := agronomy.PredictYieldPerHa(model, featuresAt(-5, baselineYield)); got < 0 {
		t.Errorf("PredictYieldPerHa = %v, want at least 0", got)
	}
}

func TestPredictYieldRangeWithoutHistoryIsTheVarietyReferenceRange(t *testing.T) {
	variety := agronomy.Variety{YieldPerHaMin: 4.0, YieldPerHaMax: 7.0, GddRequirement: 1500}
	model := agronomy.FitYieldModel(nil)
	features := agronomy.YieldFeatures{VarietyBaselineYieldPerHa: 5.5, GddRatio: 1, AreaHa: 1, MeanTempC: 27}

	got := agronomy.PredictYieldRange(model, features, variety)

	if math.Abs(got.Low-4.0) > 1e-9 || math.Abs(got.High-7.0) > 1e-9 {
		t.Fatalf("model tanpa panen harus mengaku tidak tahu selebar acuan varietas, dapat %+v", got)
	}
}

func TestPredictYieldRangeIsAlwaysOrdered(t *testing.T) {
	variety := agronomy.Variety{YieldPerHaMin: 4.0, YieldPerHaMax: 7.0, GddRequirement: 1500}
	features := agronomy.YieldFeatures{VarietyBaselineYieldPerHa: 5.5, GddRatio: 1.1, AreaHa: 0.8, MeanTempC: 28}

	for _, count := range []int{0, 1, 3, 10, 40} {
		observations := make([]agronomy.YieldObservation, 0, count)
		for i := range count {
			observations = append(observations, agronomy.YieldObservation{
				ActualYieldPerHa: 5.5 + float64(i%3)*0.4,
				Features: agronomy.YieldFeatures{
					VarietyBaselineYieldPerHa: 5.5,
					GddRatio:                  1 + float64(i)*0.01,
					AreaHa:                    0.8,
					MeanTempC:                 27.5 + float64(i%4)*0.2,
				},
			})
		}

		got := agronomy.PredictYieldRange(agronomy.FitYieldModel(observations), features, variety)

		if !(got.Low <= got.Mid && got.Mid <= got.High) {
			t.Fatalf("n=%d rentang tidak terurut: %+v", count, got)
		}
		if got.Low < 0 {
			t.Fatalf("n=%d batas bawah negatif: %+v", count, got)
		}
	}
}

func TestPredictYieldRangeNarrowsAsHarvestsAccumulate(t *testing.T) {
	variety := agronomy.Variety{YieldPerHaMin: 4.0, YieldPerHaMax: 7.0, GddRequirement: 1500}
	features := agronomy.YieldFeatures{VarietyBaselineYieldPerHa: 5.5, GddRatio: 1, AreaHa: 1, MeanTempC: 27}

	consistent := make([]agronomy.YieldObservation, 0, 30)
	for i := range 30 {
		consistent = append(consistent, agronomy.YieldObservation{
			ActualYieldPerHa: 5.5,
			Features: agronomy.YieldFeatures{
				VarietyBaselineYieldPerHa: 5.5,
				GddRatio:                  1 + float64(i)*0.005,
				AreaHa:                    1,
				MeanTempC:                 27,
			},
		})
	}

	cold := agronomy.PredictYieldRange(agronomy.FitYieldModel(nil), features, variety)
	warm := agronomy.PredictYieldRange(agronomy.FitYieldModel(consistent), features, variety)

	if warm.High-warm.Low >= cold.High-cold.Low {
		t.Fatalf("panen yang konsisten harus mempersempit rentang: dingin %+v hangat %+v", cold, warm)
	}
}

func TestPredictYieldPerHaTreatsAnUnfittedModelAsNoHistory(t *testing.T) {
	features := agronomy.YieldFeatures{VarietyBaselineYieldPerHa: 5.5, GddRatio: 1, AreaHa: 1, MeanTempC: 27}

	got := agronomy.PredictYieldPerHa(agronomy.YieldModel{}, features)

	if math.Abs(got-5.5) > 1e-9 {
		t.Fatalf("model kosong harus mengembalikan acuan varietas, dapat %v", got)
	}
}

func TestPredictYieldRangeSurvivesAnUnfittedModel(t *testing.T) {
	variety := agronomy.Variety{YieldPerHaMin: 4, YieldPerHaMax: 7}
	features := agronomy.YieldFeatures{VarietyBaselineYieldPerHa: 5.5, GddRatio: 1, AreaHa: 1, MeanTempC: 27}

	got := agronomy.PredictYieldRange(agronomy.YieldModel{}, features, variety)

	if math.Abs(got.Low-4) > 1e-9 || math.Abs(got.High-7) > 1e-9 {
		t.Fatalf("model kosong harus melebar ke acuan varietas, dapat %+v", got)
	}
}
