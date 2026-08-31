package route_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"terrion-backend/internal/delivery/http/route"
)

func TestSetupRegistersTheWeatherCronRouteBehindItsSecret(t *testing.T) {
	app := fiber.New()
	config := route.RouteConfig{App: app, CronSecret: "s3cret"}
	config.Setup()

	request := httptest.NewRequest(http.MethodPost, "/api/cron/weather", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d: an unauthenticated cron call must not reach the handler",
			response.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestSetupLeavesUnknownPathsAlone(t *testing.T) {
	app := fiber.New()
	config := route.RouteConfig{App: app, CronSecret: "s3cret"}
	config.Setup()

	request := httptest.NewRequest(http.MethodGet, "/api/nothing-here", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d", response.StatusCode, fiber.StatusNotFound)
	}
}
