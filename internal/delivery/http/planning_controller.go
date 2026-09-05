package http

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/model"
	"terrion-backend/internal/model/converter"
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

func (c *PlanningController) Propose(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	seasonLabel := ctx.Query("season")
	if seasonLabel == "" {
		return fiber.NewError(fiber.StatusBadRequest, "season is required")
	}

	proposal, err := c.UseCase.Propose(
		ctx.UserContext(), *user.CooperativeID, seasonLabel, time.Now())
	if err != nil {
		return c.failure(ctx, "proposing a plan", err)
	}

	return ctx.JSON(model.WebResponse[*model.ProposalResponse]{
		Data: converter.ProposalToResponse(proposal),
	})
}

func (c *PlanningController) Apply(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	request := new(model.ApplySeasonPlanRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	applied, err := c.UseCase.Apply(ctx.UserContext(), user, request, time.Now())
	if err != nil {
		return c.failure(ctx, "applying a plan", err)
	}

	return ctx.Status(fiber.StatusCreated).
		JSON(model.WebResponse[*model.ApplySeasonPlanResponse]{
			Data: &model.ApplySeasonPlanResponse{
				PlanID: applied.PlanID,
				Blocks: applied.Blocks,
			},
		})
}

func (c *PlanningController) Cancel(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	cancelled, err := c.UseCase.Cancel(
		ctx.UserContext(), user, ctx.Params("id"), time.Now())
	if err != nil {
		return c.failure(ctx, "cancelling a plan", err)
	}

	return ctx.JSON(model.WebResponse[*model.CancelSeasonPlanResponse]{
		Data: &model.CancelSeasonPlanResponse{
			PlanID:        cancelled.PlanID,
			BlocksRemoved: cancelled.Blocks,
		},
	})
}

func (c *PlanningController) Get(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	stored, err := c.UseCase.Get(ctx.UserContext(), user, ctx.Params("id"))
	if err != nil {
		return c.failure(ctx, "reading a plan", err)
	}

	return ctx.JSON(model.WebResponse[*model.SeasonPlanResponse]{
		Data: converter.StoredPlanToResponse(stored),
	})
}

func (c *PlanningController) List(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	plans, err := c.UseCase.List(ctx.UserContext(), user)
	if err != nil {
		return c.failure(ctx, "listing plans", err)
	}

	return ctx.JSON(model.WebResponse[*model.SeasonPlanListResponse]{
		Data: converter.SeasonPlansToResponse(plans),
	})
}

func (c *PlanningController) failure(ctx *fiber.Ctx, action string, err error) error {
	var refusal *usecase.PlanRefusal
	if errors.As(err, &refusal) {
		return fiber.NewError(statusForRefusal(refusal.Code), refusal.Code)
	}

	var validationError validator.ValidationErrors
	if errors.As(err, &validationError) {
		return fiber.NewError(fiber.StatusBadRequest, validationError.Error())
	}
	if errors.Is(err, usecase.ErrNoCooperative) {
		return fiber.NewError(fiber.StatusForbidden, "account is not linked to a cooperative")
	}

	c.Log.Errorf("%s: %v", action, err)
	return fiber.NewError(fiber.StatusInternalServerError, "request failed")
}

func statusForRefusal(code string) int {
	if code == constants.PlanNotFound {
		return fiber.StatusNotFound
	}
	return fiber.StatusUnprocessableEntity
}
