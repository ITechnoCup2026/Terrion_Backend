package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
)

func CronSecret(secret string) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		if secret == "" {
			return fiber.NewError(fiber.StatusServiceUnavailable, "CRON_SECRET is not configured")
		}

		provided := []byte(ctx.Get(fiber.HeaderAuthorization))
		expected := []byte("Bearer " + secret)
		if subtle.ConstantTimeCompare(provided, expected) != 1 {
			return fiber.NewError(fiber.StatusUnauthorized, "Unauthorised")
		}

		return ctx.Next()
	}
}
