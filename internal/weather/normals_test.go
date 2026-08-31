package weather_test

import (
	"math"
	"testing"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/weather"
)

func day(date string, tmin, tmax float64) agronomy.TempDay {
	return agronomy.TempDay{Date: date, TMin: tmin, TMax: tmax}
}

func deriveOrFail(t *testing.T, days []agronomy.TempDay) []agronomy.ClimateNormal {
	t.Helper()
	normals, err := weather.DeriveNormals(days)
	if err != nil {
		t.Fatalf("DeriveNormals: %v", err)
	}
	return normals
}

func threeNewYears() []agronomy.TempDay {
	return []agronomy.TempDay{
		day("2023-01-01", 20, 28),
		day("2024-01-01", 22, 30),
		day("2025-01-01", 24, 32),
	}
}

func TestDeriveNormalsAveragesTheDailyMeanAcrossYears(t *testing.T) {
	normals := deriveOrFail(t, threeNewYears())

	if len(normals) != 1 {
		t.Fatalf("len(normals) = %d, want 1", len(normals))
	}
	if normals[0].DayOfYear != 1 {
		t.Errorf("DayOfYear = %d, want 1", normals[0].DayOfYear)
	}
	if math.Abs(normals[0].MeanC-26) > 5e-6 {
		t.Errorf("MeanC = %v, want 26", normals[0].MeanC)
	}
}

func TestDeriveNormalsReportsYearToYearSpread(t *testing.T) {
	normals := deriveOrFail(t, threeNewYears())

	if math.Abs(normals[0].SdC-2) > 5e-6 {
		t.Errorf("SdC = %v, want 2", normals[0].SdC)
	}
}

func TestDeriveNormalsEmitsOneEntryPerDayOfYearInOrder(t *testing.T) {
	normals := deriveOrFail(t, []agronomy.TempDay{
		day("2024-03-01", 20, 30),
		day("2024-01-01", 20, 30),
		day("2024-02-01", 20, 30),
	})

	want := []int{1, 32, 61}
	if len(normals) != len(want) {
		t.Fatalf("len(normals) = %d, want %d", len(normals), len(want))
	}
	for i, normal := range normals {
		if normal.DayOfYear != want[i] {
			t.Errorf("normals[%d].DayOfYear = %d, want %d", i, normal.DayOfYear, want[i])
		}
	}
}

func TestDeriveNormalsReportsZeroSpreadForASingleYear(t *testing.T) {
	normals := deriveOrFail(t, []agronomy.TempDay{day("2024-01-01", 20, 30)})

	if normals[0].SdC != 0 {
		t.Errorf("SdC = %v, want 0", normals[0].SdC)
	}
}

func TestDeriveNormalsOfNoWeatherIsEmpty(t *testing.T) {
	if normals := deriveOrFail(t, nil); len(normals) != 0 {
		t.Errorf("len(normals) = %d, want 0", len(normals))
	}
}

func TestDeriveNormalsRejectsAMalformedDate(t *testing.T) {
	if _, err := weather.DeriveNormals([]agronomy.TempDay{day("01-01-2024", 20, 30)}); err == nil {
		t.Error("DeriveNormals returned nil error, want a parse failure")
	}
}
