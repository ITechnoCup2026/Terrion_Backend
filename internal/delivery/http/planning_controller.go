package http

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/model"
	"terrion-backend/internal/usecase"
)

type PlanningController struct {
	UseCase *usecase.PlanningUseCase
	Log     *logrus.Logger
}

func NewPlanningController(
	planningUseCase *usecase.PlanningUseCase, log *logrus.Logger,
) *PlanningController {
	return &PlanningController{UseCase: planningUseCase, Log: log}
}

func cooperativeOf(ctx *fiber.Ctx) (string, error) {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}
	if user.CooperativeID == nil {
		return "", fiber.NewError(fiber.StatusForbidden, "no cooperative")
	}
	return *user.CooperativeID, nil
}

func (c *PlanningController) Propose(ctx *fiber.Ctx) error {
	cooperativeID, err := cooperativeOf(ctx)
	if err != nil {
		return err
	}

	request := new(model.ProposePlanRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	proposal, err := c.UseCase.Propose(ctx.UserContext(), cooperativeID, request, time.Now())
	if err != nil {
		return c.failure(err)
	}

	return ctx.JSON(model.WebResponse[*model.ProposePlanResponse]{Data: proposal})
}

func (c *PlanningController) Apply(ctx *fiber.Ctx) error {
	cooperativeID, err := cooperativeOf(ctx)
	if err != nil {
		return err
	}

	request := new(model.ApplyPlanRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	applied, err := c.UseCase.Apply(ctx.UserContext(), cooperativeID, request)
	if err != nil {
		return c.failure(err)
	}

	return ctx.Status(fiber.StatusCreated).
		JSON(model.WebResponse[*model.ApplyPlanResponse]{Data: applied})
}

func (c *PlanningController) Cancel(ctx *fiber.Ctx) error {
	cooperativeID, err := cooperativeOf(ctx)
	if err != nil {
		return err
	}

	if err := c.UseCase.Cancel(ctx.UserContext(), cooperativeID, ctx.Params("id")); err != nil {
		return c.failure(err)
	}

	return ctx.SendStatus(fiber.StatusNoContent)
}

func (c *PlanningController) failure(err error) error {
	switch {
	case errors.Is(err, usecase.ErrNoPlots):
		return fiber.NewError(fiber.StatusUnprocessableEntity, usecase.ErrNoPlots.Error())
	case errors.Is(err, usecase.ErrNoCandidates):
		return fiber.NewError(fiber.StatusUnprocessableEntity, usecase.ErrNoCandidates.Error())
	case errors.Is(err, usecase.ErrSeasonInvalid):
		return fiber.NewError(fiber.StatusBadRequest, usecase.ErrSeasonInvalid.Error())
	case errors.Is(err, usecase.ErrPlanNotFound):
		return fiber.NewError(fiber.StatusNotFound, usecase.ErrPlanNotFound.Error())
	}

	c.Log.WithError(err).Warn("planning_request_failed")
	return fiber.NewError(fiber.StatusBadRequest, err.Error())
}
