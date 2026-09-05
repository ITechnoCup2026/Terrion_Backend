package http

import (
	"errors"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/catalog"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/model"
	"terrion-backend/internal/model/converter"
	"terrion-backend/internal/usecase"
)

type CatalogController struct {
	UseCase       *usecase.CatalogUseCase
	SupplyRequest *usecase.SupplyRequestUseCase
	Log           *logrus.Logger
}

func NewCatalogController(
	catalogUseCase *usecase.CatalogUseCase,
	supplyRequestUseCase *usecase.SupplyRequestUseCase,
	log *logrus.Logger,
) *CatalogController {
	return &CatalogController{
		UseCase:       catalogUseCase,
		SupplyRequest: supplyRequestUseCase,
		Log:           log,
	}
}

func (c *CatalogController) Get(ctx *fiber.Ctx) error {
	now := time.Now()

	built, err := c.UseCase.Load(ctx.UserContext(), horizonFromQuery(ctx), now)
	if err != nil {
		c.Log.Errorf("loading catalogue: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load catalogue")
	}

	built.Listings = catalog.FilterListings(built.Listings, filtersFromQuery(ctx), now)

	return ctx.JSON(model.WebResponse[*model.CatalogResponse]{
		Data: converter.CatalogToResponse(built),
	})
}

func (c *CatalogController) GetForCooperative(ctx *fiber.Ctx) error {
	listings, err := c.UseCase.LoadForCooperative(
		ctx.UserContext(), ctx.Params("id"), horizonFromQuery(ctx), time.Now())
	if err != nil {
		c.Log.Errorf("loading cooperative listings: %v", err)
		return fiber.NewError(fiber.StatusNotFound, "cooperative not found")
	}

	return ctx.JSON(model.WebResponse[[]model.ListingResponse]{
		Data: converter.ListingsToResponse(listings),
	})
}

func (c *CatalogController) ListRequests(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	requests, err := c.SupplyRequest.List(ctx.UserContext(), user)
	if err != nil {
		c.Log.Errorf("listing supply requests: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list requests")
	}

	return ctx.JSON(model.WebResponse[[]model.SupplyRequestResponse]{
		Data: converter.SupplyRequestsToResponse(requests),
	})
}

func (c *CatalogController) CreateRequest(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	request := new(model.CreateSupplyRequestRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	stored, err := c.SupplyRequest.Create(ctx.UserContext(), user, request, time.Now())
	if err != nil {
		return c.requestFailure(err, "creating supply request")
	}

	return ctx.Status(fiber.StatusCreated).JSON(model.WebResponse[*model.SupplyRequestResponse]{
		Data: converter.SupplyRequestToResponse(stored),
	})
}

func (c *CatalogController) RespondToRequest(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	request := new(model.RespondToRequestRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	err := c.SupplyRequest.Respond(
		ctx.UserContext(), user, ctx.Params("id"), request, time.Now())
	if err != nil {
		return c.requestFailure(err, "answering supply request")
	}

	return ctx.SendStatus(fiber.StatusNoContent)
}

func (c *CatalogController) requestFailure(err error, what string) error {
	var validationError validator.ValidationErrors
	if errors.As(err, &validationError) {
		return fiber.NewError(fiber.StatusBadRequest, validationError.Error())
	}
	if errors.Is(err, usecase.ErrListingUnknown) {
		return fiber.NewError(fiber.StatusBadRequest, constants.ListingUnknown)
	}
	if errors.Is(err, usecase.ErrListingGone) {
		return fiber.NewError(fiber.StatusUnprocessableEntity, constants.ListingGone)
	}
	if errors.Is(err, usecase.ErrRequestNotFound) {
		return fiber.NewError(fiber.StatusNotFound, constants.RequestNotFound)
	}
	if errors.Is(err, usecase.ErrAllocationExceeded) {
		return fiber.NewError(fiber.StatusConflict, constants.AllocationExceeded)
	}
	if errors.Is(err, usecase.ErrNoCooperative) {
		return fiber.NewError(fiber.StatusForbidden, "account is not linked to a cooperative")
	}

	c.Log.Errorf("%s: %v", what, err)
	return fiber.NewError(fiber.StatusInternalServerError, "request failed")
}

func filtersFromQuery(ctx *fiber.Ctx) catalog.Filters {
	filters := catalog.Filters{
		CommodityID: ctx.Query("commodity_id"),
		Province:    ctx.Query("province"),
	}

	if weeks, err := strconv.Atoi(ctx.Query("weeks_ahead")); err == nil {
		filters.WeeksAhead = weeks
	}
	if minTonnes, err := strconv.ParseFloat(ctx.Query("min_tonnes"), 64); err == nil {
		filters.MinTonnes = minTonnes
	}

	return filters
}
