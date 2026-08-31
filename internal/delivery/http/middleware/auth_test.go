package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/delivery/http/middleware"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/supabase"
	"terrion-backend/internal/usecase"
)

const (
	jwtSecret  = "middleware-test-secret"
	kaderID    = "11111111-1111-4111-8111-111111111111"
	strangerID = "99999999-9999-4999-8999-999999999999"
)

func authUseCase(t *testing.T) *usecase.AuthUseCase {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.AppUser{}); err != nil {
		t.Fatalf("migrating app_user: %v", err)
	}

	cooperativeID := "22222222-2222-4222-8222-222222222222"
	kader := &entity.AppUser{
		ID:            kaderID,
		Role:          constants.RoleKader,
		CooperativeID: &cooperativeID,
		FullName:      "Bu Sri",
	}
	if err := db.Create(kader).Error; err != nil {
		t.Fatalf("seeding app_user: %v", err)
	}

	log := logrus.New()
	log.SetOutput(io.Discard)

	return usecase.NewAuthUseCase(db, log, validator.New(),
		&repository.Repository[entity.AppUser]{}, supabase.NewVerifier(jwtSecret), nil)
}

func accessToken(t *testing.T, subject string) string {
	t.Helper()

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": subject,
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(jwtSecret))
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return token
}

func authedApp(t *testing.T) *fiber.App {
	t.Helper()

	app := fiber.New()
	app.Get("/api/me", middleware.Auth(authUseCase(t)), func(ctx *fiber.Ctx) error {
		user := middleware.AuthenticatedUser(ctx)
		if user == nil {
			return ctx.SendString("no user in locals")
		}
		return ctx.SendString(user.ID)
	})
	return app
}

func callWithAuth(t *testing.T, app *fiber.App, path, authorization string) (int, string) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		request.Header.Set(fiber.HeaderAuthorization, authorization)
	}

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return response.StatusCode, string(body)
}

func TestAuthPutsTheProfileInLocals(t *testing.T) {
	app := authedApp(t)

	status, body := callWithAuth(t, app, "/api/me",
		constants.BearerPrefix+accessToken(t, kaderID))

	if status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if body != kaderID {
		t.Errorf("body = %q, want the authenticated user id", body)
	}
}

func TestAuthRejectsTokensItCannotUse(t *testing.T) {
	app := authedApp(t)

	tests := []struct {
		name          string
		authorization string
	}{
		{"no header", ""},
		{"no bearer scheme", accessToken(t, kaderID)},
		{"garbage token", constants.BearerPrefix + "not-a-token"},
		{"wrong secret", constants.BearerPrefix + "a.b.c"},
		{"valid token with no profile", constants.BearerPrefix + accessToken(t, strangerID)},
	}

	for _, test := range tests {
		status, _ := callWithAuth(t, app, "/api/me", test.authorization)
		if status != fiber.StatusUnauthorized {
			t.Errorf("%s: status = %d, want %d", test.name, status, fiber.StatusUnauthorized)
		}
	}
}

func roleGuardedApp(roles ...constants.UserRole) *fiber.App {
	app := fiber.New()
	app.Get("/guarded", func(ctx *fiber.Ctx) error {
		if role := ctx.Query("role"); role != "" {
			ctx.Locals(constants.AuthUserLocal, &entity.AppUser{
				ID: kaderID, Role: constants.UserRole(role),
			})
		}
		return ctx.Next()
	}, middleware.RequireRole(roles...), func(ctx *fiber.Ctx) error {
		return ctx.SendString("allowed")
	})
	return app
}

func TestRequireRoleAllowsAnyListedRole(t *testing.T) {
	app := roleGuardedApp(constants.RoleKader, constants.RolePengurus)

	for _, role := range []constants.UserRole{constants.RoleKader, constants.RolePengurus} {
		status, _ := callWithAuth(t, app, "/guarded?role="+string(role), "")
		if status != fiber.StatusOK {
			t.Errorf("role %q: status = %d, want %d", role, status, fiber.StatusOK)
		}
	}
}

func TestRequireRoleRejectsARoleNotOnTheList(t *testing.T) {
	app := roleGuardedApp(constants.RoleKader, constants.RolePengurus)

	status, _ := callWithAuth(t, app, "/guarded?role=buyer", "")
	if status != fiber.StatusForbidden {
		t.Errorf("status = %d, want %d", status, fiber.StatusForbidden)
	}
}

func TestRequireRoleRejectsAnUnauthenticatedRequest(t *testing.T) {
	app := roleGuardedApp(constants.RoleKader)

	status, _ := callWithAuth(t, app, "/guarded", "")
	if status != fiber.StatusForbidden {
		t.Errorf("status = %d, want %d", status, fiber.StatusForbidden)
	}
}

func TestRequireRoleWithAnEmptyListDeniesEveryone(t *testing.T) {
	app := roleGuardedApp()

	for _, role := range []constants.UserRole{
		constants.RoleKader, constants.RolePengurus, constants.RoleBuyer,
	} {
		status, _ := callWithAuth(t, app, "/guarded?role="+string(role), "")
		if status != fiber.StatusForbidden {
			t.Errorf("role %q: status = %d, want %d", role, status, fiber.StatusForbidden)
		}
	}
}
