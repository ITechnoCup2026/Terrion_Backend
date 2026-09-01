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
	UseCase    *usecase.AuthUseCase
	Log        *logrus.Logger
	Production bool
}

func NewAuthController(authUseCase *usecase.AuthUseCase, log *logrus.Logger, production bool) *AuthController {
	return &AuthController{UseCase: authUseCase, Log: log, Production: production}
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

func (c *AuthController) Login(ctx *fiber.Ctx) error {
	request := new(model.LoginRequest)
	if err := ctx.BodyParser(request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, constants.SignupErrorValidation)
	}

	sessionID, user, err := c.UseCase.Login(ctx.UserContext(), request)
	if err != nil {
		return c.loginFailure(err)
	}

	c.setSessionCookie(ctx, sessionID)
	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: converter.UserToResponse(user)})
}

func (c *AuthController) loginFailure(err error) error {
	var authError *supabase.AuthError
	if errors.As(err, &authError) {
		return fiber.NewError(fiber.StatusBadRequest, authError.Code)
	}

	var validationError validator.ValidationErrors
	if errors.As(err, &validationError) {
		return fiber.NewError(fiber.StatusBadRequest, constants.SignupErrorValidation)
	}

	c.Log.Errorf("login failed: %v", err)
	return fiber.NewError(fiber.StatusInternalServerError, constants.SignupErrorUnavailable)
}

func (c *AuthController) Refresh(ctx *fiber.Ctx) error {
	sessionID := ctx.Cookies(constants.SessionCookieName)

	if err := c.UseCase.RefreshSession(ctx.UserContext(), sessionID); err != nil {
		if errors.Is(err, usecase.ErrUnauthorised) {
			c.clearSessionCookie(ctx)
			return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
		}
		c.Log.Errorf("refresh failed: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, constants.SignupErrorUnavailable)
	}

	return ctx.SendStatus(fiber.StatusNoContent)
}

func (c *AuthController) Logout(ctx *fiber.Ctx) error {
	sessionID := ctx.Cookies(constants.SessionCookieName)

	if err := c.UseCase.Logout(ctx.UserContext(), sessionID); err != nil {
		c.Log.Errorf("logout failed: %v", err)
	}

	c.clearSessionCookie(ctx)
	return ctx.SendStatus(fiber.StatusNoContent)
}

func (c *AuthController) setSessionCookie(ctx *fiber.Ctx, sessionID string) {
	ctx.Cookie(&fiber.Cookie{
		Name:     constants.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(constants.SessionTTL.Seconds()),
		HTTPOnly: true,
		Secure:   c.Production,
		SameSite: c.sameSite(),
	})
}

func (c *AuthController) clearSessionCookie(ctx *fiber.Ctx) {
	ctx.Cookie(&fiber.Cookie{
		Name:     constants.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   c.Production,
		SameSite: c.sameSite(),
	})
}

func (c *AuthController) sameSite() string {
	if c.Production {
		return fiber.CookieSameSiteNoneMode
	}
	return fiber.CookieSameSiteLaxMode
}
