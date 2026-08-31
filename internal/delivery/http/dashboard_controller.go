package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/model"
	"terrion-backend/internal/model/converter"
	"terrion-backend/internal/usecase"
)

type DashboardController struct {
	UseCase *usecase.DashboardUseCase
	Log     *logrus.Logger
}

func NewDashboardController(
	dashboardUseCase *usecase.DashboardUseCase, log *logrus.Logger,
) *DashboardController {
	return &DashboardController{UseCase: dashboardUseCase, Log: log}
}

func (c *DashboardController) Get(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	loaded, err := c.UseCase.Load(ctx.UserContext(), *user.CooperativeID, time.Now())
	if err != nil {
		c.Log.Errorf("loading dashboard: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load dashboard")
	}

	return ctx.JSON(model.WebResponse[*model.DashboardResponse]{
		Data: converter.DashboardToResponse(loaded),
	})
}
