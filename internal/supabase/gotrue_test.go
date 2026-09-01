package supabase_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"terrion-backend/internal/supabase"
)

type capturedCall struct {
	method     string
	path       string
	query      string
	apiKey     string
	authHeader string
}

func fakeGoTrue(t *testing.T, status int, body string) (*supabase.Client, *capturedCall) {
	t.Helper()

	captured := &capturedCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.query = r.URL.RawQuery
		captured.apiKey = r.Header.Get("apikey")
		captured.authHeader = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fake response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := supabase.NewClient(server.URL, "anon-key", "service-key")
	return client, captured
}

func TestSignUpReportsASessionWhenConfirmationIsOff(t *testing.T) {
	client, captured := fakeGoTrue(t, http.StatusOK, `{
		"access_token":"jwt-here",
		"user":{"id":"user-1","identities":[{"id":"identity-1"}]}}`)

	result, err := client.SignUp(context.Background(), "diana@example.com", "hunter2hunter2")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if result.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", result.UserID)
	}
	if !result.HasSession {
		t.Error("HasSession = false, want true when an access token came back")
	}
	if result.AlreadyRegistered {
		t.Error("AlreadyRegistered = true, want false")
	}
	if captured.path != "/auth/v1/signup" || captured.method != http.MethodPost {
		t.Errorf("called %s %s, want POST /auth/v1/signup", captured.method, captured.path)
	}
	if captured.apiKey != "anon-key" || captured.authHeader != "Bearer anon-key" {
		t.Errorf("signup used %q/%q, want the anon key so email confirmation applies",
			captured.apiKey, captured.authHeader)
	}
}

func TestSignUpReportsNoSessionWhenConfirmationIsOn(t *testing.T) {
	client, _ := fakeGoTrue(t, http.StatusOK,
		`{"id":"user-2","identities":[{"id":"identity-2"}]}`)

	result, err := client.SignUp(context.Background(), "diana@example.com", "hunter2hunter2")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if result.UserID != "user-2" {
		t.Errorf("UserID = %q, want user-2", result.UserID)
	}
	if result.HasSession {
		t.Error("HasSession = true, want false when no access token came back")
	}
}

func TestSignUpDetectsAnAlreadyRegisteredAddress(t *testing.T) {
	client, _ := fakeGoTrue(t, http.StatusOK, `{"id":"user-3","identities":[]}`)

	result, err := client.SignUp(context.Background(), "taken@example.com", "hunter2hunter2")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if !result.AlreadyRegistered {
		t.Error("AlreadyRegistered = false, want true for an empty identities list")
	}
}

func TestSignUpTreatsAbsentIdentitiesAsANewAccount(t *testing.T) {
	client, _ := fakeGoTrue(t, http.StatusOK, `{"id":"user-4"}`)

	result, err := client.SignUp(context.Background(), "new@example.com", "hunter2hunter2")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if result.AlreadyRegistered {
		t.Error("AlreadyRegistered = true, want false when identities is absent entirely")
	}
}

func TestSignUpSurfacesTheGoTrueErrorCode(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"error_code field", `{"code":400,"error_code":"weak_password","msg":"..."}`, "weak_password"},
		{"string code field", `{"code":"email_address_invalid"}`, "email_address_invalid"},
		{"legacy error field", `{"error":"signup_disabled"}`, "signup_disabled"},
		{"nothing usable", `{}`, "internal"},
	}

	for _, test := range tests {
		client, _ := fakeGoTrue(t, http.StatusBadRequest, test.body)

		_, err := client.SignUp(context.Background(), "diana@example.com", "hunter2hunter2")

		var authError *supabase.AuthError
		if !errors.As(err, &authError) {
			t.Errorf("%s: error = %v, want a *supabase.AuthError", test.name, err)
			continue
		}
		if authError.Code != test.want {
			t.Errorf("%s: Code = %q, want %q", test.name, authError.Code, test.want)
		}
	}
}

func TestSignUpFailsWhenNoUserComesBack(t *testing.T) {
	client, _ := fakeGoTrue(t, http.StatusOK, `{}`)

	if _, err := client.SignUp(context.Background(), "diana@example.com", "hunter2hunter2"); err == nil {
		t.Error("SignUp returned nil error for a response with no user, want a failure")
	}
}

func TestDeleteUserUsesTheServiceRoleKey(t *testing.T) {
	client, captured := fakeGoTrue(t, http.StatusOK, `{}`)

	if err := client.DeleteUser(context.Background(), "user-1"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	if captured.method != http.MethodDelete || captured.path != "/auth/v1/admin/users/user-1" {
		t.Errorf("called %s %s, want DELETE /auth/v1/admin/users/user-1",
			captured.method, captured.path)
	}
	if captured.apiKey != "service-key" || captured.authHeader != "Bearer service-key" {
		t.Errorf("delete used %q/%q, want the service role key",
			captured.apiKey, captured.authHeader)
	}
}

func TestDeleteUserReportsAFailure(t *testing.T) {
	client, _ := fakeGoTrue(t, http.StatusNotFound, `{"error_code":"user_not_found"}`)

	if err := client.DeleteUser(context.Background(), "ghost"); err == nil {
		t.Error("DeleteUser returned nil error for a 404, want a failure")
	}
}

func TestSignInReturnsTheSession(t *testing.T) {
	client, captured := fakeGoTrue(t, http.StatusOK, `{
		"access_token":"access-1",
		"refresh_token":"refresh-1",
		"user":{"id":"user-1"}}`)

	session, err := client.SignIn(context.Background(), "diana@example.com", "hunter2hunter2")
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	if session.UserID != "user-1" || session.AccessToken != "access-1" || session.RefreshToken != "refresh-1" {
		t.Errorf("session = %+v, want user-1/access-1/refresh-1", session)
	}
	if captured.path != "/auth/v1/token" || captured.method != http.MethodPost {
		t.Errorf("called %s %s, want POST /auth/v1/token", captured.method, captured.path)
	}
	if captured.query != "grant_type=password" {
		t.Errorf("query = %q, want grant_type=password", captured.query)
	}
	if captured.apiKey != "anon-key" || captured.authHeader != "Bearer anon-key" {
		t.Errorf("signin used %q/%q, want the anon key", captured.apiKey, captured.authHeader)
	}
}

func TestSignInSurfacesInvalidCredentials(t *testing.T) {
	client, _ := fakeGoTrue(t, http.StatusBadRequest, `{"error_code":"invalid_credentials"}`)

	_, err := client.SignIn(context.Background(), "diana@example.com", "wrong-password")

	var authError *supabase.AuthError
	if !errors.As(err, &authError) {
		t.Fatalf("error = %v, want a *supabase.AuthError", err)
	}
	if authError.Code != "invalid_credentials" {
		t.Errorf("Code = %q, want invalid_credentials", authError.Code)
	}
}

func TestRefreshTokenRotatesTheSession(t *testing.T) {
	client, captured := fakeGoTrue(t, http.StatusOK, `{
		"access_token":"access-2",
		"refresh_token":"refresh-2",
		"user":{"id":"user-1"}}`)

	session, err := client.RefreshToken(context.Background(), "refresh-1")
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}

	if session.AccessToken != "access-2" || session.RefreshToken != "refresh-2" {
		t.Errorf("session = %+v, want the rotated pair", session)
	}
	if captured.query != "grant_type=refresh_token" {
		t.Errorf("query = %q, want grant_type=refresh_token", captured.query)
	}
	if captured.apiKey != "anon-key" || captured.authHeader != "Bearer anon-key" {
		t.Errorf("refresh used %q/%q, want the anon key", captured.apiKey, captured.authHeader)
	}
}

func TestLogoutUsesTheUsersAccessTokenNotTheAnonKey(t *testing.T) {
	client, captured := fakeGoTrue(t, http.StatusNoContent, "")

	if err := client.Logout(context.Background(), "user-access-token"); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if captured.path != "/auth/v1/logout" || captured.method != http.MethodPost {
		t.Errorf("called %s %s, want POST /auth/v1/logout", captured.method, captured.path)
	}
	if captured.apiKey != "anon-key" {
		t.Errorf("apiKey = %q, want the anon key", captured.apiKey)
	}
	if captured.authHeader != "Bearer user-access-token" {
		t.Errorf("authHeader = %q, want the user's own access token, not the anon key", captured.authHeader)
	}
}

func TestLogoutReportsAFailure(t *testing.T) {
	client, _ := fakeGoTrue(t, http.StatusUnauthorized, `{"error_code":"session_not_found"}`)

	if err := client.Logout(context.Background(), "stale-token"); err == nil {
		t.Error("Logout returned nil error for a 401, want a failure")
	}
}
