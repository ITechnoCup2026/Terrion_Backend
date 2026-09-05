package repository

import (
	"testing"

	"terrion-backend/internal/entity"
	"terrion-backend/internal/weather"
)

func TestFindNormalsForCellsGroupsEveryRequestedCell(t *testing.T) {
	db := setupWeatherDB(t)

	first := weather.GridCell{GridLat: -6.25, GridLng: 107.75}
	second := weather.GridCell{GridLat: -6.25, GridLng: 106.75}
	absent := weather.GridCell{GridLat: -7.25, GridLng: 110.75}
	crossed := weather.GridCell{GridLat: -6.25, GridLng: 110.75}

	rows := []entity.WeatherNormal{}
	for _, cell := range []weather.GridCell{first, second, crossed} {
		for day := 1; day <= 3; day++ {
			rows = append(rows, entity.WeatherNormal{
				GridLat: cell.GridLat, GridLng: cell.GridLng,
				DayOfYear: day, MeanC: 26, SdC: 1,
			})
		}
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seeding normals: %v", err)
	}

	repo := &WeatherRepository{}
	normals, err := repo.FindNormalsForCells(db, []weather.GridCell{first, second, absent})
	if err != nil {
		t.Fatalf("FindNormalsForCells: %v", err)
	}

	if len(normals[first]) != 3 {
		t.Errorf("normals[first] = %d rows, want 3", len(normals[first]))
	}
	if len(normals[second]) != 3 {
		t.Errorf("normals[second] = %d rows, want 3", len(normals[second]))
	}
	if _, present := normals[absent]; present {
		t.Errorf("normals[absent] present, want the cell to be missing")
	}
	if _, present := normals[crossed]; present {
		t.Errorf("normals[crossed] present, want cells matched as pairs not as separate columns")
	}
}

func TestFindNormalsForCellsOrdersByDayOfYear(t *testing.T) {
	db := setupWeatherDB(t)
	cell := weather.GridCell{GridLat: -6.25, GridLng: 107.75}

	for _, day := range []int{9, 2, 5} {
		if err := db.Create(&entity.WeatherNormal{
			GridLat: cell.GridLat, GridLng: cell.GridLng,
			DayOfYear: day, MeanC: 26, SdC: 1,
		}).Error; err != nil {
			t.Fatalf("seeding normal %d: %v", day, err)
		}
	}

	normals, err := (&WeatherRepository{}).
		FindNormalsForCells(db, []weather.GridCell{cell})
	if err != nil {
		t.Fatalf("FindNormalsForCells: %v", err)
	}

	want := []int{2, 5, 9}
	for i, row := range normals[cell] {
		if row.DayOfYear != want[i] {
			t.Errorf("normals[%d].DayOfYear = %d, want %d", i, row.DayOfYear, want[i])
		}
	}
}
