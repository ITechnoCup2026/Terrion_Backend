package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/delivery/http"
	"terrion-backend/internal/delivery/http/route"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/usecase"
	"terrion-backend/internal/weather"
)

type BootstrapConfig struct {
	DB       *gorm.DB
	Redis    *redis.Client
	App      *fiber.App
	Log      *logrus.Logger
	Validate *validator.Validate
	Config   *Config
}

func Bootstrap(bootstrapConfig *BootstrapConfig) {
	weatherRepository := &repository.WeatherRepository{}

	weatherUseCase := usecase.NewWeatherUseCase(
		bootstrapConfig.DB, bootstrapConfig.Log, weatherRepository, weather.NewClient())

	weatherController := http.NewWeatherController(weatherUseCase, bootstrapConfig.Log)

	routeConfig := route.RouteConfig{
		App:               bootstrapConfig.App,
		WeatherController: weatherController,
		CronSecret:        bootstrapConfig.Config.Cron.Secret,
	}
	routeConfig.Setup()
}
