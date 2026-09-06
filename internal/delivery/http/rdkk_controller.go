package http

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
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

	season, err := seasonFromQuery(ctx)
	if err != nil {
		return err
	}

	var request *model.CreateInputOrderRequest
	if len(ctx.Body()) > 0 {
		request = new(model.CreateInputOrderRequest)
		if err := ctx.BodyParser(request); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
		}
	}

	created, err := c.UseCase.CreateInputOrder(ctx.UserContext(), user, season, request)
	if err != nil {
		return c.orderFailure(err, "creating input order")
	}

	return ctx.Status(fiber.StatusCreated).
		JSON(model.WebResponse[*model.CreateInputOrderResponse]{
			Data: &model.CreateInputOrderResponse{
				OrderID: created.OrderID,
				Lines:   created.Lines,
			},
		})
}

func (c *RdkkController) ListInputOrders(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	orders, err := c.UseCase.ListInputOrders(ctx.UserContext(), user)
	if err != nil {
		c.Log.Errorf("listing input orders: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list input orders")
	}

	return ctx.JSON(model.WebResponse[[]model.InputOrderResponse]{
		Data: converter.InputOrdersToResponse(orders),
	})
}

func (c *RdkkController) UpdateInputOrderStatus(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	request := new(model.UpdateInputOrderStatusRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	err := c.UseCase.UpdateInputOrderStatus(
		ctx.UserContext(), user, ctx.Params("id"), request, time.Now())
	if err != nil {
		return c.orderFailure(err, "updating input order status")
	}

	return ctx.SendStatus(fiber.StatusNoContent)
}

func (c *RdkkController) orderFailure(err error, what string) error {
	var validationError validator.ValidationErrors
	if errors.As(err, &validationError) {
		return fiber.NewError(fiber.StatusBadRequest, validationError.Error())
	}
	if errors.Is(err, usecase.ErrNoCooperative) {
		return fiber.NewError(fiber.StatusForbidden, "account is not linked to a cooperative")
	}
	if errors.Is(err, usecase.ErrNothingToOrder) {
		return fiber.NewError(fiber.StatusUnprocessableEntity, constants.RdkkNothingToOrder)
	}
	if errors.Is(err, usecase.ErrOrderSeasonAlreadyOpen) {
		return fiber.NewError(fiber.StatusConflict, constants.OrderSeasonAlreadyOpen)
	}
	if errors.Is(err, usecase.ErrOrderLineUnknown) {
		return fiber.NewError(fiber.StatusUnprocessableEntity, constants.OrderLineUnknown)
	}
	if errors.Is(err, usecase.ErrOrderLinesEmpty) {
		return fiber.NewError(fiber.StatusUnprocessableEntity, constants.OrderLinesEmpty)
	}
	if errors.Is(err, usecase.ErrOrderNotFound) {
		return fiber.NewError(fiber.StatusNotFound, constants.OrderNotFound)
	}
	if errors.Is(err, usecase.ErrOrderTransitionInvalid) {
		return fiber.NewError(fiber.StatusConflict, constants.OrderTransitionInvalid)
	}
	if errors.Is(err, usecase.ErrOrderAlreadyFinal) {
		return fiber.NewError(fiber.StatusConflict, constants.OrderAlreadyFinal)
	}
	c.Log.Errorf("%s: %v", what, err)
	return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("failed to %s", what))
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
