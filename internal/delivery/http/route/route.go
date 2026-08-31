package route

import (
	"github.com/gofiber/fiber/v2"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/delivery/http"
	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/usecase"
)

type RouteConfig struct {
	App               *fiber.App
	AuthController    *http.AuthController
	PlotController    *http.PlotController
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
	c.App.Get("/api/commodities", c.PlotController.Commodities)
}

func (c *RouteConfig) setupAuthenticatedRoutes() {
	auth := middleware.Auth(c.AuthUseCase)

	fieldStaff := middleware.RequireRole(constants.RoleKader, constants.RolePengurus)

	c.App.Get("/api/me", auth, c.AuthController.Me)
	c.App.Get("/api/plots", auth, c.PlotController.List)
	c.App.Get("/api/plots/:id", auth, c.PlotController.Get)
	c.App.Post("/api/plots", auth, fieldStaff, c.PlotController.Create)
	c.App.Post("/api/blocks/:id/split", auth, fieldStaff, c.PlotController.SplitBlock)
}

func (c *RouteConfig) setupCronRoutes() {
	cron := c.App.Group("/api/cron", middleware.CronSecret(c.CronSecret))
	cron.Post("/weather", c.WeatherController.Refresh)
}
