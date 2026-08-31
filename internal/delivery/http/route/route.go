package route

import (
	"github.com/gofiber/fiber/v2"

	"terrion-backend/internal/delivery/http"
	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/usecase"
)

type RouteConfig struct {
	App               *fiber.App
	AuthController    *http.AuthController
	WeatherController *http.WeatherController
	AuthUseCase       *usecase.AuthUseCase
	CronSecret        string
}

func (c *RouteConfig) Setup() {
	c.setupPublicRoutes()
	c.setupAuthenticatedRoutes()
	c.setupCronRoutes()
}

func (c *RouteConfig) setupPublicRoutes() {
	c.App.Post("/api/auth/signup", c.AuthController.SignUp)
}

func (c *RouteConfig) setupAuthenticatedRoutes() {
	auth := middleware.Auth(c.AuthUseCase)

	c.App.Get("/api/me", auth, c.AuthController.Me)
}

func (c *RouteConfig) setupCronRoutes() {
	cron := c.App.Group("/api/cron", middleware.CronSecret(c.CronSecret))
	cron.Post("/weather", c.WeatherController.Refresh)
}
