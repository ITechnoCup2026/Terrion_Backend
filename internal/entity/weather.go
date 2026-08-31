package entity

import "time"

// WeatherDaily holds both observed readings and the 16-day forecast, keyed by
// grid cell rather than by plot: one download serves every plot in the cell.
// A forecast row is overwritten by the observation once the day arrives.
type WeatherDaily struct {
	GridLat float64   `gorm:"column:grid_lat;primaryKey"`
	GridLng float64   `gorm:"column:grid_lng;primaryKey"`
	Date    time.Time `gorm:"column:date;primaryKey"`
	TempMin float64   `gorm:"column:temp_min"`
	TempMax float64   `gorm:"column:temp_max"`
}

func (WeatherDaily) TableName() string { return "weather_daily" }

// WeatherNormal is ten years collapsed into one typical year. SdC is the entire
// source of a harvest window's width: a cell whose SdC is zero predicts a
// single date.
type WeatherNormal struct {
	GridLat   float64 `gorm:"column:grid_lat;primaryKey"`
	GridLng   float64 `gorm:"column:grid_lng;primaryKey"`
	DayOfYear int     `gorm:"column:day_of_year;primaryKey"`
	MeanC     float64 `gorm:"column:mean_c"`
	SdC       float64 `gorm:"column:sd_c"`
}

func (WeatherNormal) TableName() string { return "weather_normals" }
