package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/weather"
)

type WeatherRepository struct {
	Repository[entity.WeatherDaily]
}

func (r *WeatherRepository) UpsertDaily(db *gorm.DB, rows []entity.WeatherDaily) error {
	if len(rows) == 0 {
		return nil
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "grid_lat"}, {Name: "grid_lng"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"temp_min", "temp_max"}),
	}).CreateInBatches(rows, constants.WeatherUpsertBatchSize).Error
}

func (r *WeatherRepository) UpsertNormals(db *gorm.DB, rows []entity.WeatherNormal) error {
	if len(rows) == 0 {
		return nil
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "grid_lat"}, {Name: "grid_lng"}, {Name: "day_of_year"}},
		DoUpdates: clause.AssignmentColumns([]string{"mean_c", "sd_c"}),
	}).CreateInBatches(rows, constants.WeatherUpsertBatchSize).Error
}

func (r *WeatherRepository) CountDaily(db *gorm.DB, cell weather.GridCell) (int64, error) {
	var total int64
	err := db.Model(&entity.WeatherDaily{}).
		Where("grid_lat = ? AND grid_lng = ?", cell.GridLat, cell.GridLng).
		Count(&total).Error
	return total, err
}

func (r *WeatherRepository) FindDailySince(
	db *gorm.DB, cell weather.GridCell, since time.Time,
) ([]entity.WeatherDaily, error) {
	rows := []entity.WeatherDaily{}
	err := db.Where("grid_lat = ? AND grid_lng = ? AND date >= ?",
		cell.GridLat, cell.GridLng, since).
		Order("date").
		Find(&rows).Error
	return rows, err
}

func (r *WeatherRepository) FindNormals(
	db *gorm.DB, cell weather.GridCell,
) ([]entity.WeatherNormal, error) {
	rows := []entity.WeatherNormal{}
	err := db.Where("grid_lat = ? AND grid_lng = ?", cell.GridLat, cell.GridLng).
		Order("day_of_year").
		Find(&rows).Error
	return rows, err
}

func (r *WeatherRepository) FindOccupiedGridCells(db *gorm.DB) ([]weather.GridCell, error) {
	cells := []weather.GridCell{}
	err := db.Model(&entity.Plot{}).
		Select("DISTINCT grid_lat, grid_lng").
		Where("grid_lat IS NOT NULL AND grid_lng IS NOT NULL").
		Order("grid_lat, grid_lng").
		Scan(&cells).Error
	return cells, err
}
