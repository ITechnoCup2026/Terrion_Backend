package entity

import "time"

type WeatherDaily struct {
	GridLat float64   `gorm:"column:grid_lat;primaryKey"`
	GridLng float64   `gorm:"column:grid_lng;primaryKey"`
	Date    time.Time `gorm:"column:date;primaryKey"`
	TempMin float64   `gorm:"column:temp_min"`
	TempMax float64   `gorm:"column:temp_max"`
}

func (WeatherDaily) TableName() string { return "weather_daily" }

type WeatherNormal struct {
	GridLat   float64 `gorm:"column:grid_lat;primaryKey"`
	GridLng   float64 `gorm:"column:grid_lng;primaryKey"`
	DayOfYear int     `gorm:"column:day_of_year;primaryKey"`
	MeanC     float64 `gorm:"column:mean_c"`
	SdC       float64 `gorm:"column:sd_c"`
}

func (WeatherNormal) TableName() string { return "weather_normals" }
