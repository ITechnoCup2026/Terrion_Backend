package route

import (
	"github.com/gofiber/fiber/v2"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/delivery/http"
	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/model"
	"terrion-backend/internal/usecase"
)

type RouteConfig struct {
	App                 *fiber.App
	ServiceName         string
	AuthController      *http.AuthController
	CatalogController   *http.CatalogController
	DashboardController *http.DashboardController
	PlotController      *http.PlotController
	PublicController    *http.PublicController
	RdkkController      *http.RdkkController
	StaggerController   *http.StaggerController
	WeatherController   *http.WeatherController
	AuthUseCase         *usecase.AuthUseCase
	CronSecret          string
}

func (c *RouteConfig) Setup() {
	c.setupPublicRoutes()
	c.setupAuthenticatedRoutes()
	c.setupCronRoutes()
}

func (c *RouteConfig) setupPublicRoutes() {
	c.App.Get("/api/health", c.health)
	c.App.Post("/api/auth/signup", c.AuthController.SignUp)
	c.App.Post("/api/auth/login", c.AuthController.Login)
	c.App.Post("/api/auth/refresh", c.AuthController.Refresh)
	c.App.Post("/api/auth/logout", c.AuthController.Logout)
	c.App.Get("/api/commodities", c.PlotController.Commodities)
	c.App.Get("/api/catalog", c.CatalogController.Get)
	c.App.Get("/api/catalog/cooperatives/:id", c.CatalogController.GetForCooperative)
	c.App.Get("/api/public/plots/:publicId", c.PublicController.GetPlot)
	c.App.Get("/api/atlas/cooperatives", c.PublicController.Cooperatives)
	c.App.Get("/api/atlas/farms/:id", c.PublicController.Farm)
}

func (c *RouteConfig) setupAuthenticatedRoutes() {
	auth := middleware.Auth(c.AuthUseCase)

	fieldStaff := middleware.RequireRole(constants.RoleKader, constants.RolePengurus)

	c.App.Get("/api/me", auth, c.AuthController.Me)
	c.App.Get("/api/dashboard", auth, c.DashboardController.Get)
	c.App.Get("/api/plots", auth, c.PlotController.List)
	c.App.Get("/api/plots/:id", auth, c.PlotController.Get)
	c.App.Post("/api/plots", auth, fieldStaff, c.PlotController.Create)
	c.App.Post("/api/blocks/:id/split", auth, fieldStaff, c.PlotController.SplitBlock)
	c.App.Get("/api/rdkk", auth, c.RdkkController.Get)
	c.App.Post("/api/input-orders", auth,
		middleware.RequireRole(constants.RolePengurus), c.RdkkController.CreateInputOrder)
	c.App.Get("/api/input-orders", auth, c.RdkkController.ListInputOrders)
	c.App.Get("/api/supply-requests", auth, c.CatalogController.ListRequests)
	c.App.Post("/api/supply-requests", auth,
		middleware.RequireRole(constants.RoleBuyer), c.CatalogController.CreateRequest)
	c.App.Patch("/api/supply-requests/:id", auth,
		middleware.RequireRole(constants.RolePengurus), c.CatalogController.RespondToRequest)
	c.App.Post("/api/stagger", auth,
		middleware.RequireRole(constants.RolePengurus), c.StaggerController.Apply)
}

func (c *RouteConfig) setupCronRoutes() {
	cron := c.App.Group("/api/cron", middleware.CronSecret(c.CronSecret))
	cron.Post("/weather", c.WeatherController.Refresh)
}

func (c *RouteConfig) health(ctx *fiber.Ctx) error {
	return ctx.JSON(model.WebResponse[*model.HealthResponse]{
		Data: &model.HealthResponse{Status: "ok", Service: c.ServiceName},
	})
}
