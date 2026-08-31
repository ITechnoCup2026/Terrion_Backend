package agronomy_test

import (
	"math"
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
)

var (
	seasonStart = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	paddy = agronomy.Variety{
		GddRequirement: 2000, BaseTempC: 12,
		DaysToHarvestMin: 110, DaysToHarvestMax: 125,
		YieldPerHaMin: 5, YieldPerHaMax: 7,
	}
)

func flatWeather(from time.Time, days int, meanC float64) []agronomy.TempDay {
	weather := make([]agronomy.TempDay, days)
	for i := range weather {
		weather[i] = agronomy.TempDay{
			Date: agronomy.ToISODate(agronomy.AddDays(from, i)),
			TMin: meanC - 5,
			TMax: meanC + 5,
		}
	}
	return weather
}

func harvestedBlock(weather []agronomy.TempDay) agronomy.YieldObservationInput {
	return agronomy.YieldObservationInput{
		PlantingDate:  seasonStart,
		HarvestDate:   agronomy.AddDays(seasonStart, 99),
		AreaHa:        2,
		ActualYieldKg: 12000,
		Variety:       paddy,
		Weather:       weather,
	}
}

func observationOrFail(t *testing.T, input agronomy.YieldObservationInput) agronomy.YieldObservation {
	t.Helper()
	observation, ok := agronomy.DeriveYieldObservation(input)
	if !ok {
		t.Fatal("DeriveYieldObservation returned not ok, want an observation")
	}
	return observation
}

func TestDeriveYieldObservationComputesAchievedGddRatio(t *testing.T) {
	observation := observationOrFail(t, harvestedBlock(flatWeather(seasonStart, 100, 27)))

	closeTo(t, "GddRatio", observation.Features.GddRatio, 0.75, 5e-6)
}

func TestDeriveYieldObservationConvertsKilogramsToTonnesPerHectare(t *testing.T) {
	observation := observationOrFail(t, harvestedBlock(flatWeather(seasonStart, 100, 27)))

	closeTo(t, "ActualYieldPerHa", observation.ActualYieldPerHa, 6, 5e-6)
	closeTo(t, "VarietyBaselineYieldPerHa", observation.Features.VarietyBaselineYieldPerHa, 6, 5e-6)
}

func TestDeriveYieldObservationAveragesTemperatureOverTheSeasonOnly(t *testing.T) {
	weather := flatWeather(agronomy.AddDays(seasonStart, -30), 30, 40)
	weather = append(weather, flatWeather(seasonStart, 100, 27)...)
	weather = append(weather, flatWeather(agronomy.AddDays(seasonStart, 100), 30, 40)...)

	observation := observationOrFail(t, harvestedBlock(weather))

	closeTo(t, "MeanTempC", observation.Features.MeanTempC, 27, 5e-6)
}

func TestDeriveYieldObservationCountsOnlyInSeasonWeatherTowardGdd(t *testing.T) {
	weather := flatWeather(seasonStart, 100, 27)
	weather = append(weather, flatWeather(agronomy.AddDays(seasonStart, 100), 200, 27)...)

	observation := observationOrFail(t, harvestedBlock(weather))

	closeTo(t, "GddRatio", observation.Features.GddRatio, 0.75, 5e-6)
}

func TestDeriveYieldObservationRejectsASeasonWithNoWeather(t *testing.T) {
	if _, ok := agronomy.DeriveYieldObservation(harvestedBlock(nil)); ok {
		t.Error("DeriveYieldObservation returned ok, want rejection")
	}
}

func TestDeriveYieldObservationRejectsZeroArea(t *testing.T) {
	input := harvestedBlock(flatWeather(seasonStart, 100, 27))
	input.AreaHa = 0

	if _, ok := agronomy.DeriveYieldObservation(input); ok {
		t.Error("DeriveYieldObservation returned ok, want rejection")
	}
}

func TestDeriveYieldFeaturesMeasuresHeatSoFar(t *testing.T) {
	features := agronomy.DeriveYieldFeatures(agronomy.YieldFeaturesInput{
		PlantingDate: seasonStart,
		ThroughDate:  agronomy.AddDays(seasonStart, 49),
		AreaHa:       2,
		Variety:      paddy,
		Weather:      flatWeather(seasonStart, 100, 27),
	})

	closeTo(t, "GddRatio", features.GddRatio, 0.375, 5e-6)
	closeTo(t, "MeanTempC", features.MeanTempC, 27, 5e-6)
	closeTo(t, "VarietyBaselineYieldPerHa", features.VarietyBaselineYieldPerHa, 6, 5e-6)
}

func TestDeriveYieldFeaturesReportsZeroHeatBeforeAnyWeatherArrives(t *testing.T) {
	features := agronomy.DeriveYieldFeatures(agronomy.YieldFeaturesInput{
		PlantingDate: seasonStart,
		ThroughDate:  seasonStart,
		AreaHa:       1,
		Variety:      paddy,
	})

	if features.GddRatio != 0 {
		t.Errorf("GddRatio = %v, want 0", features.GddRatio)
	}
	if math.IsNaN(features.MeanTempC) || math.IsInf(features.MeanTempC, 0) {
		t.Errorf("MeanTempC = %v, want a finite number", features.MeanTempC)
	}
}
