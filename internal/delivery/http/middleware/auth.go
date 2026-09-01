package middleware

import (
	"slices"

	"github.com/gofiber/fiber/v2"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/usecase"
)

func Auth(authUseCase *usecase.AuthUseCase) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		user, err := authUseCase.Authenticate(ctx.UserContext(), ctx.Cookies(constants.SessionCookieName))
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
		}

		ctx.Locals(constants.AuthUserLocal, user)
		return ctx.Next()
	}
}

func RequireRole(roles ...constants.UserRole) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		user := AuthenticatedUser(ctx)
		if user == nil || !slices.Contains(roles, user.Role) {
			return fiber.NewError(fiber.StatusForbidden, "Forbidden")
		}
		return ctx.Next()
	}
}

func AuthenticatedUser(ctx *fiber.Ctx) *entity.AppUser {
	user, ok := ctx.Locals(constants.AuthUserLocal).(*entity.AppUser)
	if !ok {
		return nil
	}
	return user
}
