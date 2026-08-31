package http

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/model"
	"terrion-backend/internal/model/converter"
	"terrion-backend/internal/usecase"
)

type PublicController struct {
	Public *usecase.PublicUseCase
	Atlas  *usecase.AtlasUseCase
	Log    *logrus.Logger
}

func NewPublicController(
	publicUseCase *usecase.PublicUseCase, atlasUseCase *usecase.AtlasUseCase,
	log *logrus.Logger,
) *PublicController {
	return &PublicController{Public: publicUseCase, Atlas: atlasUseCase, Log: log}
}

func (c *PublicController) GetPlot(ctx *fiber.Ctx) error {
	plot, err := c.Public.LoadPlot(ctx.UserContext(), ctx.Params("publicId"), time.Now())
	if err != nil {
		if errors.Is(err, usecase.ErrPublicPlotNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "plot not found")
		}
		c.Log.Errorf("loading public plot: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load plot")
	}

	return ctx.JSON(model.WebResponse[*model.PublicPlotResponse]{
		Data: converter.PublicPlotToResponse(plot),
	})
}

func (c *PublicController) Cooperatives(ctx *fiber.Ctx) error {
	cooperatives, err := c.Atlas.Cooperatives(ctx.UserContext())
	if err != nil {
		c.Log.Errorf("loading atlas cooperatives: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load the atlas")
	}

	return ctx.JSON(model.WebResponse[[]model.AtlasCooperativeResponse]{
		Data: converter.AtlasCooperativesToResponse(cooperatives),
	})
}

func (c *PublicController) Farm(ctx *fiber.Ctx) error {
	farm, err := c.Atlas.Farm(ctx.UserContext(), ctx.Params("id"))
	if err != nil {
		if errors.Is(err, usecase.ErrCooperativeNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "cooperative not found")
		}
		c.Log.Errorf("loading atlas farm: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load the farm")
	}

	return ctx.JSON(model.WebResponse[*model.AtlasFarmResponse]{
		Data: converter.AtlasFarmToResponse(farm),
	})
}
