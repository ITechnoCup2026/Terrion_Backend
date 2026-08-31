package route

import (
	"github.com/gofiber/fiber/v2"

	"terrion-backend/internal/delivery/http"
	"terrion-backend/internal/delivery/http/middleware"
)

type RouteConfig struct {
	App               *fiber.App
	WeatherController *http.WeatherController
	CronSecret        string
}

func (c *RouteConfig) Setup() {
	c.setupCronRoutes()
}

func (c *RouteConfig) setupCronRoutes() {
	cron := c.App.Group("/api/cron", middleware.CronSecret(c.CronSecret))
	cron.Post("/weather", c.WeatherController.Refresh)
}
