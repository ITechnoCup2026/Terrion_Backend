package http

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/model/converter"
	"terrion-backend/internal/plots"
	"terrion-backend/internal/usecase"
)

type PlotController struct {
	UseCase *usecase.PlotUseCase
	Log     *logrus.Logger
}

func NewPlotController(plotUseCase *usecase.PlotUseCase, log *logrus.Logger) *PlotController {
	return &PlotController{UseCase: plotUseCase, Log: log}
}

func (c *PlotController) List(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	summaries, err := c.UseCase.List(ctx.UserContext(), *user.CooperativeID, time.Now())
	if err != nil {
		c.Log.Errorf("listing plots: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list plots")
	}

	return ctx.JSON(model.WebResponse[[]model.PlotSummaryResponse]{
		Data: converter.PlotSummariesToResponse(summaries),
	})
}

func (c *PlotController) Get(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	detail, err := c.UseCase.Get(
		ctx.UserContext(), *user.CooperativeID, ctx.Params("id"), time.Now())
	if err != nil {
		if errors.Is(err, usecase.ErrPlotNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "plot not found")
		}
		c.Log.Errorf("reading plot: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to read plot")
	}

	return ctx.JSON(model.WebResponse[*model.PlotDetailResponse]{
		Data: converter.PlotDetailToResponse(detail),
	})
}

func (c *PlotController) Create(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	request := new(model.CreatePlotRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	created, err := c.UseCase.Create(ctx.UserContext(), user, request)
	if err != nil {
		return c.writeFailure(err, "creating plot")
	}

	return ctx.Status(fiber.StatusCreated).JSON(model.WebResponse[*model.CreatePlotResponse]{
		Data: &model.CreatePlotResponse{PlotID: created.PlotID, PublicID: created.PublicID},
	})
}

func (c *PlotController) SplitBlock(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	request := new(model.SplitBlockRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	result, err := c.UseCase.SplitBlock(ctx.UserContext(), user, ctx.Params("id"), request)
	if err != nil {
		var refusal *plots.SplitRefusal
		if errors.As(err, &refusal) {
			return ctx.Status(fiber.StatusUnprocessableEntity).
				JSON(model.WebResponse[*model.SplitRefusalResponse]{
					Errors: refusal.Code,
					Data: &model.SplitRefusalResponse{
						MinHa:         refusal.MinHa,
						BlockAreaHa:   refusal.BlockAreaHa,
						MaxTakeableHa: refusal.MaxTakeableHa,
					},
				})
		}
		return c.writeFailure(err, "splitting block")
	}

	return ctx.Status(fiber.StatusCreated).JSON(model.WebResponse[*model.SplitBlockResponse]{
		Data: &model.SplitBlockResponse{PlotID: result.PlotID, BlockID: result.BlockID},
	})
}

func (c *PlotController) Commodities(ctx *fiber.Ctx) error {
	commodities, varieties, err := c.UseCase.Catalogue(ctx.UserContext())
	if err != nil {
		c.Log.Errorf("reading commodity catalogue: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to read commodities")
	}

	return ctx.JSON(model.WebResponse[[]model.CommodityResponse]{
		Data: converter.CommodityCatalogueToResponse(commodities, varieties),
	})
}

func (c *PlotController) writeFailure(err error, what string) error {
	var validationError validator.ValidationErrors
	if errors.As(err, &validationError) {
		return fiber.NewError(fiber.StatusBadRequest, validationError.Error())
	}
	if errors.Is(err, usecase.ErrNoCooperative) {
		return fiber.NewError(fiber.StatusForbidden, "account is not linked to a cooperative")
	}
	if errors.Is(err, usecase.ErrAreaTooLarge) {
		return fiber.NewError(fiber.StatusBadRequest, "plantings exceed the maximum plot area")
	}

	c.Log.Errorf("%s: %v", what, err)
	return fiber.NewError(fiber.StatusInternalServerError, "request failed")
}

func cooperativeMember(ctx *fiber.Ctx) (*entity.AppUser, error) {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}
	if user.CooperativeID == nil {
		return nil, fiber.NewError(fiber.StatusForbidden, "account is not linked to a cooperative")
	}
	return user, nil
}
