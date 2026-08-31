package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/model"
	"terrion-backend/internal/model/converter"
	"terrion-backend/internal/usecase"
)

type WeatherController struct {
	UseCase *usecase.WeatherUseCase
	Log     *logrus.Logger
}

func NewWeatherController(
	weatherUseCase *usecase.WeatherUseCase, log *logrus.Logger,
) *WeatherController {
	return &WeatherController{UseCase: weatherUseCase, Log: log}
}

func (c *WeatherController) Refresh(ctx *fiber.Ctx) error {
	result, err := c.UseCase.RefreshAllGrids(ctx.UserContext(), time.Now())
	if err != nil {
		c.Log.Errorf("refreshing weather grids: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to refresh weather")
	}

	return ctx.JSON(model.WebResponse[*model.WeatherRefreshResponse]{
		Data: converter.WeatherRefreshToResponse(result),
	})
}
