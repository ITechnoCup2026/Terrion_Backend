package config

import (
	"terrion-backend/internal/delivery/http/route"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type BootstrapConfig struct {
	DB       *gorm.DB
	App      *fiber.App
	Log      *logrus.Logger
	Validate *validator.Validate
	Config   *Config
}

// Bootstrap is the composition root: it wires repositories, use cases,
// controllers, and middleware together, then hands them to the route
// config. As domains are added, construct them here in that order
// (repository -> usecase -> controller) before calling routeConfig.Setup().
func Bootstrap(bootstrapConfig *BootstrapConfig) {
	routeConfig := route.RouteConfig{
		App: bootstrapConfig.App,
	}
	routeConfig.Setup()
}
