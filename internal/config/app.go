package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/delivery/http"
	"terrion-backend/internal/delivery/http/route"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/supabase"
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
	userRepository := &repository.Repository[entity.AppUser]{}
	weatherRepository := &repository.WeatherRepository{}

	goTrue := supabase.NewClient(
		bootstrapConfig.Config.Supabase.URL,
		bootstrapConfig.Config.Supabase.AnonKey,
		bootstrapConfig.Config.Supabase.ServiceRoleKey,
	)
	verifier := supabase.NewVerifier(bootstrapConfig.Config.Supabase.JWTSecret)

	authUseCase := usecase.NewAuthUseCase(
		bootstrapConfig.DB, bootstrapConfig.Log, bootstrapConfig.Validate,
		userRepository, verifier, goTrue)

	weatherUseCase := usecase.NewWeatherUseCase(
		bootstrapConfig.DB, bootstrapConfig.Log, weatherRepository, weather.NewClient())

	authController := http.NewAuthController(authUseCase, bootstrapConfig.Log)
	weatherController := http.NewWeatherController(weatherUseCase, bootstrapConfig.Log)

	routeConfig := route.RouteConfig{
		App:               bootstrapConfig.App,
		AuthController:    authController,
		WeatherController: weatherController,
		AuthUseCase:       authUseCase,
		CronSecret:        bootstrapConfig.Config.Cron.Secret,
	}
	routeConfig.Setup()
}
