package http

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/model"
	"terrion-backend/internal/model/converter"
	"terrion-backend/internal/usecase"
)

type RdkkController struct {
	UseCase *usecase.RdkkUseCase
	Log     *logrus.Logger
}

func NewRdkkController(rdkkUseCase *usecase.RdkkUseCase, log *logrus.Logger) *RdkkController {
	return &RdkkController{UseCase: rdkkUseCase, Log: log}
}

func (c *RdkkController) Get(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	season, err := seasonFromQuery(ctx)
	if err != nil {
		return err
	}

	document, err := c.UseCase.LoadSeason(ctx.UserContext(), *user.CooperativeID, season)
	if err != nil {
		c.Log.Errorf("loading RDKK: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load RDKK")
	}

	return ctx.JSON(model.WebResponse[*model.RdkkResponse]{
		Data: converter.RdkkToResponse(document, season),
	})
}

func (c *RdkkController) CreateInputOrder(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	created, err := c.UseCase.CreateInputOrder(ctx.UserContext(), user, time.Now())
	if err != nil {
		if errors.Is(err, usecase.ErrNothingToOrder) {
			return fiber.NewError(fiber.StatusUnprocessableEntity, constants.RdkkNothingToOrder)
		}
		if errors.Is(err, usecase.ErrNoCooperative) {
			return fiber.NewError(fiber.StatusForbidden, "account is not linked to a cooperative")
		}
		c.Log.Errorf("creating input order: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create input order")
	}

	return ctx.Status(fiber.StatusCreated).
		JSON(model.WebResponse[*model.CreateInputOrderResponse]{
			Data: &model.CreateInputOrderResponse{
				OrderID: created.OrderID,
				Lines:   created.Lines,
			},
		})
}

func seasonFromQuery(ctx *fiber.Ctx) (usecase.Season, error) {
	season := usecase.DefaultSeason(time.Now())

	if label := ctx.Query("label"); label != "" {
		season.Label = label
	}

	if from := ctx.Query("from"); from != "" {
		parsed, err := agronomy.UTCDate(from)
		if err != nil {
			return season, fiber.NewError(fiber.StatusBadRequest, "malformed from date")
		}
		season.Start = parsed
	}

	if to := ctx.Query("to"); to != "" {
		parsed, err := agronomy.UTCDate(to)
		if err != nil {
			return season, fiber.NewError(fiber.StatusBadRequest, "malformed to date")
		}
		season.End = parsed
	}

	if season.End.Before(season.Start) {
		return season, fiber.NewError(fiber.StatusBadRequest, "season ends before it starts")
	}

	return season, nil
}
