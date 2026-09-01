package middleware_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
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
	kaderID    = "11111111-1111-4111-8111-111111111111"
	strangerID = "99999999-9999-4999-8999-999999999999"
)

func authUseCase(t *testing.T) (*usecase.AuthUseCase, *redis.Client) {
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

	server := miniredis.RunT(t)
	cache := redis.NewClient(&redis.Options{Addr: server.Addr()})

	return usecase.NewAuthUseCase(db, log, validator.New(), cache,
		&repository.Repository[entity.AppUser]{}, supabase.NewVerifier(""), nil), cache
}

func seedSession(t *testing.T, cache *redis.Client, sessionID, userID string) {
	t.Helper()

	raw, err := json.Marshal(map[string]string{
		"user_id":       userID,
		"access_token":  "access-" + sessionID,
		"refresh_token": "refresh-" + sessionID,
	})
	if err != nil {
		t.Fatalf("marshalling seeded session: %v", err)
	}
	if err := cache.Set(context.Background(), constants.SessionKeyPrefix+sessionID, raw, 0).Err(); err != nil {
		t.Fatalf("seeding session: %v", err)
	}
}

func authedApp(t *testing.T) (*fiber.App, *redis.Client) {
	t.Helper()

	authUC, cache := authUseCase(t)

	app := fiber.New()
	app.Get("/api/me", middleware.Auth(authUC), func(ctx *fiber.Ctx) error {
		user := middleware.AuthenticatedUser(ctx)
		if user == nil {
			return ctx.SendString("no user in locals")
		}
		return ctx.SendString(user.ID)
	})
	return app, cache
}

func callWithSession(t *testing.T, app *fiber.App, path, sessionID string) (int, string) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	if sessionID != "" {
		request.AddCookie(&http.Cookie{Name: constants.SessionCookieName, Value: sessionID})
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
	app, cache := authedApp(t)
	seedSession(t, cache, "session-1", kaderID)

	status, body := callWithSession(t, app, "/api/me", "session-1")

	if status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if body != kaderID {
		t.Errorf("body = %q, want the authenticated user id", body)
	}
}

func TestAuthRejectsSessionsItCannotUse(t *testing.T) {
	app, cache := authedApp(t)
	seedSession(t, cache, "session-stranger", strangerID)

	tests := []struct {
		name      string
		sessionID string
	}{
		{"no cookie", ""},
		{"unknown session", "not-a-real-session"},
		{"session with no profile", "session-stranger"},
	}

	for _, test := range tests {
		status, _ := callWithSession(t, app, "/api/me", test.sessionID)
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
		status, _ := callWithSession(t, app, "/guarded?role="+string(role), "")
		if status != fiber.StatusOK {
			t.Errorf("role %q: status = %d, want %d", role, status, fiber.StatusOK)
		}
	}
}

func TestRequireRoleRejectsARoleNotOnTheList(t *testing.T) {
	app := roleGuardedApp(constants.RoleKader, constants.RolePengurus)

	status, _ := callWithSession(t, app, "/guarded?role=buyer", "")
	if status != fiber.StatusForbidden {
		t.Errorf("status = %d, want %d", status, fiber.StatusForbidden)
	}
}

func TestRequireRoleRejectsAnUnauthenticatedRequest(t *testing.T) {
	app := roleGuardedApp(constants.RoleKader)

	status, _ := callWithSession(t, app, "/guarded", "")
	if status != fiber.StatusForbidden {
		t.Errorf("status = %d, want %d", status, fiber.StatusForbidden)
	}
}

func TestRequireRoleWithAnEmptyListDeniesEveryone(t *testing.T) {
	app := roleGuardedApp()

	for _, role := range []constants.UserRole{
		constants.RoleKader, constants.RolePengurus, constants.RoleBuyer,
	} {
		status, _ := callWithSession(t, app, "/guarded?role="+string(role), "")
		if status != fiber.StatusForbidden {
			t.Errorf("role %q: status = %d, want %d", role, status, fiber.StatusForbidden)
		}
	}
}
