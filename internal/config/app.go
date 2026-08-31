package config

import (
	"terrion-backend/internal/delivery/http/route"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
	routeConfig := route.RouteConfig{
		App: bootstrapConfig.App,
	}
	routeConfig.Setup()
}
