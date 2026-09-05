package route_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	delivery "terrion-backend/internal/delivery/http"
	"terrion-backend/internal/delivery/http/route"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/supabase"
	"terrion-backend/internal/usecase"
)

func routedApp(t *testing.T) *fiber.App {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	authUseCase := usecase.NewAuthUseCase(nil, log, validator.New(), nil,
		&repository.Repository[entity.AppUser]{}, supabase.NewVerifier(""), nil)

	app := fiber.New()
	config := route.RouteConfig{
		App:                app,
		AuthController:     delivery.NewAuthController(authUseCase, log, false),
		WeatherController:  delivery.NewWeatherController(nil, log),
		PlanningController: delivery.NewPlanningController(nil, log),
		AuthUseCase:        authUseCase,
		CronSecret:         "s3cret",
	}
	config.Setup()
	return app
}

func statusOf(t *testing.T, app *fiber.App, method, path string, body io.Reader) int {
	t.Helper()

	request := httptest.NewRequest(method, path, body)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

func TestCronRouteIsGuardedByItsSecret(t *testing.T) {
	app := routedApp(t)

	if got := statusOf(t, app, http.MethodPost, "/api/cron/weather", nil); got != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d: an unauthenticated cron call must not reach the handler",
			got, fiber.StatusUnauthorized)
	}
}

func TestMeRequiresAnAccessToken(t *testing.T) {
	app := routedApp(t)

	if got := statusOf(t, app, http.MethodGet, "/api/me", nil); got != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d", got, fiber.StatusUnauthorized)
	}
}

func TestSignupIsReachableWithoutAToken(t *testing.T) {
	app := routedApp(t)

	body := strings.NewReader(`{"full_name":"","organisation":"","email":"","password":"","confirm_password":""}`)
	got := statusOf(t, app, http.MethodPost, "/api/auth/signup", body)

	if got == fiber.StatusUnauthorized || got == fiber.StatusNotFound {
		t.Errorf("status = %d, want the handler to be reached: signup must be public", got)
	}
}

func TestLoginIsReachableWithoutASession(t *testing.T) {
	app := routedApp(t)

	body := strings.NewReader(`{"email":"","password":""}`)
	got := statusOf(t, app, http.MethodPost, "/api/auth/login", body)

	if got == fiber.StatusUnauthorized || got == fiber.StatusNotFound {
		t.Errorf("status = %d, want the handler to be reached: login must be public", got)
	}
}

func TestRefreshAndLogoutAreReachableWithoutASession(t *testing.T) {
	app := routedApp(t)

	for _, path := range []string{"/api/auth/refresh", "/api/auth/logout"} {
		got := statusOf(t, app, http.MethodPost, path, nil)
		if got == fiber.StatusNotFound {
			t.Errorf("%s: status = %d, want the route to exist", path, got)
		}
	}
}

func TestUnknownApiPathsAreNotFound(t *testing.T) {
	app := routedApp(t)

	if got := statusOf(t, app, http.MethodGet, "/api/nothing-here", nil); got != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d: the auth guard must not swallow unrouted paths",
			got, fiber.StatusNotFound)
	}
}

func TestPlanRoutesExistAndRequireAToken(t *testing.T) {
	app := routedApp(t)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/plans"},
		{http.MethodGet, "/api/plans/propose?season=MT%20I%202026%2F2027"},
		{http.MethodGet, "/api/plans/11111111-1111-4111-8111-111111111111"},
		{http.MethodPost, "/api/plans"},
		{http.MethodPost, "/api/plans/11111111-1111-4111-8111-111111111111/cancel"},
	}

	for _, route := range cases {
		got := statusOf(t, app, route.method, route.path, nil)
		if got == fiber.StatusNotFound {
			t.Errorf("%s %s: status = %d, want the route to exist", route.method, route.path, got)
		}
		if got != fiber.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want %d",
				route.method, route.path, got, fiber.StatusUnauthorized)
		}
	}
}
