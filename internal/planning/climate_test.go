package planning_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/planning"
)

func flatNormals(meanC float64) []agronomy.ClimateNormal {
	normals := make([]agronomy.ClimateNormal, 366)
	for i := range normals {
		normals[i] = agronomy.ClimateNormal{DayOfYear: i + 1, MeanC: meanC, SdC: 1.5}
	}
	return normals
}

func lowlandRice() agronomy.Variety {
	return agronomy.Variety{
		GddRequirement: 1860, BaseTempC: 12,
		DaysToHarvestMin: 110, DaysToHarvestMax: 125,
		YieldPerHaMin: 5, YieldPerHaMax: 7,
	}
}

func TestSyntheticWeatherCoversEveryDayInclusive(t *testing.T) {
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	to := agronomy.AddDays(from, 9)

	days := planning.SyntheticWeather(flatNormals(27.8), from, to)

	if len(days) != 10 {
		t.Fatalf("len(days) = %d, want 10", len(days))
	}
	if days[0].Date != "2026-11-12" {
		t.Errorf("first date = %q, want 2026-11-12", days[0].Date)
	}
	if days[9].Date != "2026-11-21" {
		t.Errorf("last date = %q, want 2026-11-21", days[9].Date)
	}
	if days[0].TMin != 27.8 || days[0].TMax != 27.8 {
		t.Errorf("first day = (%v, %v), want (27.8, 27.8)", days[0].TMin, days[0].TMax)
	}
}

func TestSyntheticWeatherPutsYieldFeaturesInsideTrainingDistribution(t *testing.T) {
	variety := lowlandRice()
	planting := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	harvest := agronomy.AddDays(planting, 118)

	features := agronomy.DeriveYieldFeatures(agronomy.YieldFeaturesInput{
		PlantingDate: planting,
		ThroughDate:  harvest,
		AreaHa:       0.5,
		Variety:      variety,
		Weather:      planning.SyntheticWeather(flatNormals(27.8), planting, harvest),
	})

	if features.GddRatio < 0.85 || features.GddRatio > 1.15 {
		t.Errorf("GddRatio = %v, want within [0.85, 1.15]", features.GddRatio)
	}
	if features.MeanTempC < 27 || features.MeanTempC > 28.5 {
		t.Errorf("MeanTempC = %v, want near 27.8", features.MeanTempC)
	}
}

func TestSyntheticWeatherWithoutNormalsYieldsNothing(t *testing.T) {
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	days := planning.SyntheticWeather(nil, from, agronomy.AddDays(from, 30))

	if len(days) != 0 {
		t.Errorf("len(days) = %d, want 0", len(days))
	}
}
