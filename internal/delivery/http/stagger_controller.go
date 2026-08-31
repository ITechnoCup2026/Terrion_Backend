package http

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/model"
	"terrion-backend/internal/usecase"
)

type StaggerController struct {
	UseCase *usecase.StaggerUseCase
	Log     *logrus.Logger
}

func NewStaggerController(
	staggerUseCase *usecase.StaggerUseCase, log *logrus.Logger,
) *StaggerController {
	return &StaggerController{UseCase: staggerUseCase, Log: log}
}

func (c *StaggerController) Apply(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	request := new(model.ApplyStaggerRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	applied, err := c.UseCase.Apply(ctx.UserContext(), user, request, time.Now())
	if err != nil {
		return c.applyFailure(ctx, err)
	}

	return ctx.JSON(model.WebResponse[*model.ApplyStaggerResponse]{
		Data: &model.ApplyStaggerResponse{Shifted: applied.Shifted},
	})
}

func (c *StaggerController) applyFailure(ctx *fiber.Ctx, err error) error {
	var refusal *usecase.StaggerRefusal
	if errors.As(err, &refusal) {
		return ctx.Status(fiber.StatusUnprocessableEntity).
			JSON(model.WebResponse[*model.StaggerRefusalResponse]{
				Errors: refusal.Code,
				Data: &model.StaggerRefusalResponse{
					AlreadyPlanted: refusal.AlreadyPlanted,
					WouldBeInPast:  refusal.WouldBeInPast,
				},
			})
	}

	var validationError validator.ValidationErrors
	if errors.As(err, &validationError) {
		return fiber.NewError(fiber.StatusBadRequest, validationError.Error())
	}
	if errors.Is(err, usecase.ErrSuggestionStale) {
		return fiber.NewError(fiber.StatusUnprocessableEntity, constants.StaggerSuggestionStale)
	}
	if errors.Is(err, usecase.ErrNoCooperative) {
		return fiber.NewError(fiber.StatusForbidden, "account is not linked to a cooperative")
	}

	c.Log.Errorf("applying stagger: %v", err)
	return fiber.NewError(fiber.StatusInternalServerError, "request failed")
}
