package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"terrion-backend/internal/delivery/http/middleware"
)

func guardedApp(secret string) *fiber.App {
	app := fiber.New()
	app.Post("/api/cron/weather", middleware.CronSecret(secret), func(ctx *fiber.Ctx) error {
		return ctx.SendString("ran")
	})
	return app
}

func statusFor(t *testing.T, app *fiber.App, authorization string) int {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/api/cron/weather", nil)
	if authorization != "" {
		request.Header.Set(fiber.HeaderAuthorization, authorization)
	}

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

func TestCronSecretFailsClosedWhenUnconfigured(t *testing.T) {
	app := guardedApp("")

	if got := statusFor(t, app, "Bearer undefined"); got != fiber.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", got, fiber.StatusServiceUnavailable)
	}
}

func TestCronSecretRejectsAMissingOrWrongHeader(t *testing.T) {
	app := guardedApp("s3cret")

	tests := []struct {
		name          string
		authorization string
	}{
		{"missing", ""},
		{"wrong secret", "Bearer nope"},
		{"no scheme", "s3cret"},
		{"prefix only", "Bearer s3cre"},
	}

	for _, test := range tests {
		if got := statusFor(t, app, test.authorization); got != fiber.StatusUnauthorized {
			t.Errorf("%s: status = %d, want %d", test.name, got, fiber.StatusUnauthorized)
		}
	}
}

func TestCronSecretAcceptsTheConfiguredSecret(t *testing.T) {
	app := guardedApp("s3cret")

	if got := statusFor(t, app, "Bearer s3cret"); got != fiber.StatusOK {
		t.Errorf("status = %d, want %d", got, fiber.StatusOK)
	}
}
