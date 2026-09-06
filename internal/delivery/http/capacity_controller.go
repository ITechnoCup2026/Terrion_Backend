package http

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/model"
	"terrion-backend/internal/usecase"
)

type CapacityController struct {
	UseCase *usecase.CapacityUseCase
	Log     *logrus.Logger
}

func NewCapacityController(
	capacityUseCase *usecase.CapacityUseCase, log *logrus.Logger,
) *CapacityController {
	return &CapacityController{UseCase: capacityUseCase, Log: log}
}

// Get menjawab satu baris per komoditas acuan, terisi atau belum.
func (c *CapacityController) Get(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	rows, err := c.UseCase.Load(ctx.UserContext(), user)
	if err != nil {
		return c.failure(err, "reading the capacity")
	}

	return ctx.JSON(model.WebResponse[*model.CapacityResponse]{
		Data: &model.CapacityResponse{Rows: rows},
	})
}

// Put menyimpan tabel kapasitas. Pengurus saja: ambang tabrakan mengikat
// bagaimana seluruh koperasi membaca minggunya sendiri.
func (c *CapacityController) Put(ctx *fiber.Ctx) error {
	user, err := cooperativeMember(ctx)
	if err != nil {
		return err
	}

	request := new(model.SetCapacityRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "malformed request body")
	}

	if err := c.UseCase.Save(ctx.UserContext(), user, request); err != nil {
		return c.failure(err, "writing the capacity")
	}

	return ctx.SendStatus(fiber.StatusNoContent)
}

func (c *CapacityController) failure(err error, what string) error {
	var validationError validator.ValidationErrors
	if errors.As(err, &validationError) {
		return fiber.NewError(fiber.StatusBadRequest, validationError.Error())
	}
	if errors.Is(err, usecase.ErrNoCooperative) {
		return fiber.NewError(fiber.StatusForbidden, "account is not linked to a cooperative")
	}
	if errors.Is(err, usecase.ErrCommodityUnknown) {
		return fiber.NewError(
			fiber.StatusUnprocessableEntity, constants.CapacityCommodityUnknown)
	}

	c.Log.Errorf("%s: %v", what, err)
	return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("failed to %s", what))
}
