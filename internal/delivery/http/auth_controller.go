package http

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/model"
	"terrion-backend/internal/model/converter"
	"terrion-backend/internal/supabase"
	"terrion-backend/internal/usecase"
)

type AuthController struct {
	UseCase *usecase.AuthUseCase
	Log     *logrus.Logger
}

func NewAuthController(authUseCase *usecase.AuthUseCase, log *logrus.Logger) *AuthController {
	return &AuthController{UseCase: authUseCase, Log: log}
}

func (c *AuthController) Me(ctx *fiber.Ctx) error {
	user := middleware.AuthenticatedUser(ctx)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
	}

	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: converter.UserToResponse(user)})
}

func (c *AuthController) SignUp(ctx *fiber.Ctx) error {
	request := new(model.SignupRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, constants.SignupErrorValidation)
	}

	response, err := c.UseCase.SignUpBuyer(ctx.UserContext(), request)
	if err != nil {
		return c.signUpFailure(err)
	}

	return ctx.Status(fiber.StatusCreated).
		JSON(model.WebResponse[*model.SignupResponse]{Data: response})
}

func (c *AuthController) signUpFailure(err error) error {
	var authError *supabase.AuthError
	if errors.As(err, &authError) {
		return fiber.NewError(fiber.StatusBadRequest, authError.Code)
	}

	var validationError validator.ValidationErrors
	if errors.As(err, &validationError) {
		return fiber.NewError(fiber.StatusBadRequest, constants.SignupErrorValidation)
	}

	c.Log.Errorf("buyer signup failed: %v", err)
	return fiber.NewError(fiber.StatusInternalServerError, constants.SignupErrorUnavailable)
}
