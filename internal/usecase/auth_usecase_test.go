package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/supabase"
	"terrion-backend/internal/usecase"
)

const seededUserID = "user-1"

func fakeGoTrue(t *testing.T, status int, body string) *supabase.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fake response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return supabase.NewClient(server.URL, "anon-key", "service-key")
}

func testAuthUseCase(t *testing.T, goTrue *supabase.Client) (*usecase.AuthUseCase, *redis.Client) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.AppUser{}); err != nil {
		t.Fatalf("migrating app_user: %v", err)
	}
	if err := db.Create(&entity.AppUser{
		ID: seededUserID, Role: constants.RoleBuyer, FullName: "Diana",
	}).Error; err != nil {
		t.Fatalf("seeding app_user: %v", err)
	}

	log := logrus.New()
	log.SetOutput(io.Discard)

	server := miniredis.RunT(t)
	cache := redis.NewClient(&redis.Options{Addr: server.Addr()})

	authUseCase := usecase.NewAuthUseCase(db, log, validator.New(), cache,
		&repository.Repository[entity.AppUser]{}, supabase.NewVerifier(""), goTrue)
	return authUseCase, cache
}

func TestLoginCreatesASessionAndReturnsTheProfile(t *testing.T) {
	goTrue := fakeGoTrue(t, http.StatusOK, `{
		"access_token":"access-1","refresh_token":"refresh-1",
		"user":{"id":"`+seededUserID+`"}}`)
	authUseCase, cache := testAuthUseCase(t, goTrue)

	sessionID, user, err := authUseCase.Login(context.Background(), &model.LoginRequest{
		Email: "diana@example.com", Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if sessionID == "" {
		t.Fatal("sessionID is empty")
	}
	if user.ID != seededUserID {
		t.Errorf("user.ID = %q, want %q", user.ID, seededUserID)
	}

	raw, err := cache.Get(context.Background(), constants.SessionKeyPrefix+sessionID).Bytes()
	if err != nil {
		t.Fatalf("reading seeded session: %v", err)
	}
	var stored struct {
		UserID       string `json:"user_id"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decoding stored session: %v", err)
	}
	if stored.UserID != seededUserID || stored.AccessToken != "access-1" || stored.RefreshToken != "refresh-1" {
		t.Errorf("stored session = %+v, want user-1/access-1/refresh-1", stored)
	}

	ttl := cache.TTL(context.Background(), constants.SessionKeyPrefix+sessionID).Val()
	if ttl <= 0 || ttl > constants.SessionTTL {
		t.Errorf("TTL = %v, want a positive value at most %v", ttl, constants.SessionTTL)
	}
}

func TestLoginSurfacesInvalidCredentials(t *testing.T) {
	goTrue := fakeGoTrue(t, http.StatusBadRequest, `{"error_code":"invalid_credentials"}`)
	authUseCase, _ := testAuthUseCase(t, goTrue)

	_, _, err := authUseCase.Login(context.Background(), &model.LoginRequest{
		Email: "diana@example.com", Password: "wrong-password",
	})

	var authError *supabase.AuthError
	if err == nil {
		t.Fatal("Login returned nil error, want invalid_credentials")
	}
	if !errors.As(err, &authError) || authError.Code != "invalid_credentials" {
		t.Errorf("error = %v, want a *supabase.AuthError with code invalid_credentials", err)
	}
}

func TestRefreshSessionRotatesTheStoredTokens(t *testing.T) {
	goTrue := fakeGoTrue(t, http.StatusOK, `{
		"access_token":"access-1","refresh_token":"refresh-1",
		"user":{"id":"`+seededUserID+`"}}`)
	authUseCase, cache := testAuthUseCase(t, goTrue)

	sessionID, _, err := authUseCase.Login(context.Background(), &model.LoginRequest{
		Email: "diana@example.com", Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	rotated := fakeGoTrue(t, http.StatusOK, `{
		"access_token":"access-2","refresh_token":"refresh-2",
		"user":{"id":"`+seededUserID+`"}}`)
	authUseCase.GoTrue = rotated

	if err := authUseCase.RefreshSession(context.Background(), sessionID); err != nil {
		t.Fatalf("RefreshSession: %v", err)
	}

	raw, err := cache.Get(context.Background(), constants.SessionKeyPrefix+sessionID).Bytes()
	if err != nil {
		t.Fatalf("reading rotated session: %v", err)
	}
	var stored struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decoding rotated session: %v", err)
	}
	if stored.AccessToken != "access-2" || stored.RefreshToken != "refresh-2" {
		t.Errorf("stored session = %+v, want the rotated pair", stored)
	}
}

func TestRefreshSessionRejectsAnUnknownSession(t *testing.T) {
	authUseCase, _ := testAuthUseCase(t, nil)

	if err := authUseCase.RefreshSession(context.Background(), "not-a-real-session"); err != usecase.ErrUnauthorised {
		t.Errorf("err = %v, want ErrUnauthorised", err)
	}
}

func TestLogoutDeletesTheSessionAndCallsSupabase(t *testing.T) {
	goTrue := fakeGoTrue(t, http.StatusOK, `{
		"access_token":"access-1","refresh_token":"refresh-1",
		"user":{"id":"`+seededUserID+`"}}`)
	authUseCase, cache := testAuthUseCase(t, goTrue)

	sessionID, _, err := authUseCase.Login(context.Background(), &model.LoginRequest{
		Email: "diana@example.com", Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := authUseCase.Logout(context.Background(), sessionID); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if cache.Exists(context.Background(), constants.SessionKeyPrefix+sessionID).Val() != 0 {
		t.Error("session key still exists in Redis after logout")
	}
}

func TestLogoutIsIdempotentForAnUnknownSession(t *testing.T) {
	authUseCase, _ := testAuthUseCase(t, nil)

	if err := authUseCase.Logout(context.Background(), "not-a-real-session"); err != nil {
		t.Errorf("Logout: %v, want nil for an already-gone session", err)
	}
}
