package constants

import "time"

const GridStep = 0.25

const (
	WeatherHistoryYears         = 10
	WeatherBackfillCompleteRows = 3000
	WeatherRefreshLookbackDays  = 30
	WeatherUpsertBatchSize      = 1000
)

const (
	OpenMeteoArchiveURL   = "https://archive-api.open-meteo.com/v1/archive"
	OpenMeteoForecastURL  = "https://api.open-meteo.com/v1/forecast"
	OpenMeteoDailyFields  = "temperature_2m_min,temperature_2m_max"
	OpenMeteoForecastDays = 16
	OpenMeteoTimeout      = 30 * time.Second
)
