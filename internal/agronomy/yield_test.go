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
