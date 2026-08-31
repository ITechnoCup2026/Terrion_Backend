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
	plotRepository := &repository.PlotRepository{}
	blockRepository := &repository.BlockRepository{}
	memberRepository := &repository.MemberRepository{}
	commodityRepository := &repository.CommodityRepository{}
	varietyRepository := &repository.VarietyRepository{}
	calibrationRepository := &repository.CalibrationRepository{}
	cooperativeRepository := &repository.CooperativeRepository{}
	referencePriceRepository := &repository.ReferencePriceRepository{}
	inputOrderRepository := &repository.InputOrderRepository{}

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

	projectionUseCase := usecase.NewProjectionUseCase(
		bootstrapConfig.DB, bootstrapConfig.Log,
		plotRepository, blockRepository, varietyRepository, calibrationRepository,
		weatherUseCase)

	plotUseCase := usecase.NewPlotUseCase(
		bootstrapConfig.DB, bootstrapConfig.Log, bootstrapConfig.Validate,
		plotRepository, blockRepository, memberRepository,
		commodityRepository, varietyRepository, projectionUseCase, weatherUseCase)

	dashboardUseCase := usecase.NewDashboardUseCase(
		bootstrapConfig.DB, bootstrapConfig.Log,
		cooperativeRepository, blockRepository, commodityRepository, memberRepository,
		referencePriceRepository, inputOrderRepository, projectionUseCase)

	authController := http.NewAuthController(authUseCase, bootstrapConfig.Log)
	dashboardController := http.NewDashboardController(dashboardUseCase, bootstrapConfig.Log)
	plotController := http.NewPlotController(plotUseCase, bootstrapConfig.Log)
	weatherController := http.NewWeatherController(weatherUseCase, bootstrapConfig.Log)

	routeConfig := route.RouteConfig{
		App:                 bootstrapConfig.App,
		AuthController:      authController,
		DashboardController: dashboardController,
		PlotController:      plotController,
		WeatherController:   weatherController,
		AuthUseCase:         authUseCase,
		CronSecret:          bootstrapConfig.Config.Cron.Secret,
	}
	routeConfig.Setup()
}
