package repository

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
	"terrion-backend/internal/weather"
)

var subang = weather.GridCell{GridLat: -6.25, GridLng: 107.75}

func setupWeatherDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.WeatherDaily{}, &entity.WeatherNormal{}); err != nil {
		t.Fatalf("failed to migrate weather tables: %v", err)
	}
	return db
}

func dailyRow(cell weather.GridCell, date string, tempMin, tempMax float64) entity.WeatherDaily {
	parsed, _ := time.Parse("2006-01-02", date)
	return entity.WeatherDaily{
		GridLat: cell.GridLat,
		GridLng: cell.GridLng,
		Date:    parsed,
		TempMin: tempMin,
		TempMax: tempMax,
	}
}

func TestWeatherRepositoryUpsertDailyOverwritesTheSameDay(t *testing.T) {
	db := setupWeatherDB(t)
	repository := &WeatherRepository{}

	forecast := []entity.WeatherDaily{dailyRow(subang, "2026-03-02", 20, 30)}
	if err := repository.UpsertDaily(db, forecast); err != nil {
		t.Fatalf("UpsertDaily forecast: %v", err)
	}

	observed := []entity.WeatherDaily{dailyRow(subang, "2026-03-02", 22, 33)}
	if err := repository.UpsertDaily(db, observed); err != nil {
		t.Fatalf("UpsertDaily observed: %v", err)
	}

	stored, err := repository.FindDailySince(db, subang, time.Time{})
	if err != nil {
		t.Fatalf("FindDailySince: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("len(stored) = %d, want 1", len(stored))
	}
	if stored[0].TempMin != 22 || stored[0].TempMax != 33 {
		t.Errorf("stored = %+v, want the observed reading 22/33", stored[0])
	}
}

func TestWeatherRepositoryUpsertDailyKeepsCellsApart(t *testing.T) {
	db := setupWeatherDB(t)
	repository := &WeatherRepository{}
	elsewhere := weather.GridCell{GridLat: -6.75, GridLng: 107.75}

	rows := []entity.WeatherDaily{
		dailyRow(subang, "2026-03-02", 20, 30),
		dailyRow(elsewhere, "2026-03-02", 15, 24),
	}
	if err := repository.UpsertDaily(db, rows); err != nil {
		t.Fatalf("UpsertDaily: %v", err)
	}

	count, err := repository.CountDaily(db, subang)
	if err != nil {
		t.Fatalf("CountDaily: %v", err)
	}
	if count != 1 {
		t.Errorf("CountDaily(subang) = %d, want 1", count)
	}
}

func TestWeatherRepositoryFindDailySinceExcludesOlderDays(t *testing.T) {
	db := setupWeatherDB(t)
	repository := &WeatherRepository{}

	rows := []entity.WeatherDaily{
		dailyRow(subang, "2026-02-01", 20, 30),
		dailyRow(subang, "2026-03-02", 21, 31),
	}
	if err := repository.UpsertDaily(db, rows); err != nil {
		t.Fatalf("UpsertDaily: %v", err)
	}

	since, _ := time.Parse("2006-01-02", "2026-03-01")
	stored, err := repository.FindDailySince(db, subang, since)
	if err != nil {
		t.Fatalf("FindDailySince: %v", err)
	}
	if len(stored) != 1 || stored[0].TempMin != 21 {
		t.Errorf("stored = %+v, want only 2026-03-02", stored)
	}
}

func TestWeatherRepositoryUpsertNormalsReplacesACell(t *testing.T) {
	db := setupWeatherDB(t)
	repository := &WeatherRepository{}

	first := []entity.WeatherNormal{{
		GridLat: subang.GridLat, GridLng: subang.GridLng,
		DayOfYear: 1, MeanC: 26, SdC: 2,
	}}
	if err := repository.UpsertNormals(db, first); err != nil {
		t.Fatalf("UpsertNormals first: %v", err)
	}

	second := []entity.WeatherNormal{{
		GridLat: subang.GridLat, GridLng: subang.GridLng,
		DayOfYear: 1, MeanC: 27, SdC: 1,
	}}
	if err := repository.UpsertNormals(db, second); err != nil {
		t.Fatalf("UpsertNormals second: %v", err)
	}

	stored, err := repository.FindNormals(db, subang)
	if err != nil {
		t.Fatalf("FindNormals: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("len(stored) = %d, want 1", len(stored))
	}
	if stored[0].MeanC != 27 || stored[0].SdC != 1 {
		t.Errorf("stored = %+v, want mean 27 and sd 1", stored[0])
	}
}

func TestWeatherRepositoryUpsertOfNothingIsANoOp(t *testing.T) {
	db := setupWeatherDB(t)
	repository := &WeatherRepository{}

	if err := repository.UpsertDaily(db, nil); err != nil {
		t.Errorf("UpsertDaily(nil): %v", err)
	}
	if err := repository.UpsertNormals(db, nil); err != nil {
		t.Errorf("UpsertNormals(nil): %v", err)
	}
}
