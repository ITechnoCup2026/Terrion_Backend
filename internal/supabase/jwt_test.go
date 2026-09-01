package supabase_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"terrion-backend/internal/supabase"
)

const testSecret = "super-secret-jwt-signing-key"

func signedToken(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return token
}

func unsignedToken(t *testing.T, subject string) string {
	t.Helper()

	encode := func(raw string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(raw))
	}
	header := encode(`{"alg":"none","typ":"JWT"}`)
	payload := encode(`{"sub":"` + subject + `","exp":9999999999}`)
	return strings.Join([]string{header, payload, ""}, ".")
}

func TestVerifierAcceptsAValidToken(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)
	token := signedToken(t, testSecret, jwt.MapClaims{
		"sub": "11111111-1111-4111-8111-111111111111",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	subject, err := verifier.Subject(token)
	if err != nil {
		t.Fatalf("Subject: %v", err)
	}
	if subject != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("subject = %q, want the token's sub", subject)
	}
}

func TestVerifierRejectsAnUnsignedToken(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)

	if _, err := verifier.Subject(unsignedToken(t, "attacker")); err == nil {
		t.Error("Subject accepted an alg=none token, want rejection")
	}
}

func TestVerifierRejectsAWrongSecret(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)
	token := signedToken(t, "a-different-secret", jwt.MapClaims{
		"sub": "someone",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	if _, err := verifier.Subject(token); err == nil {
		t.Error("Subject accepted a token signed with another secret, want rejection")
	}
}

func TestVerifierRejectsAnExpiredToken(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)
	token := signedToken(t, testSecret, jwt.MapClaims{
		"sub": "someone",
		"exp": time.Now().Add(-time.Minute).Unix(),
	})

	if _, err := verifier.Subject(token); err == nil {
		t.Error("Subject accepted an expired token, want rejection")
	}
}

func TestVerifierRejectsATokenWithoutExpiry(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)
	token := signedToken(t, testSecret, jwt.MapClaims{"sub": "someone"})

	if _, err := verifier.Subject(token); err == nil {
		t.Error("Subject accepted a token that never expires, want rejection")
	}
}

func TestVerifierRejectsATokenWithoutSubject(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)
	token := signedToken(t, testSecret, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	if _, err := verifier.Subject(token); err == nil {
		t.Error("Subject accepted a token with no sub, want rejection")
	}
}

func TestVerifierFailsClosedWithoutASecret(t *testing.T) {
	verifier := supabase.NewVerifier("")
	token := signedToken(t, testSecret, jwt.MapClaims{
		"sub": "someone",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	if _, err := verifier.Subject(token); err == nil {
		t.Error("Subject accepted a token with no configured secret, want rejection")
	}
}

func TestVerifierRejectsGarbage(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)

	for _, token := range []string{"", "not-a-token", "a.b.c"} {
		if _, err := verifier.Subject(token); err == nil {
			t.Errorf("Subject(%q) accepted garbage, want rejection", token)
		}
	}
}

func TestSignedByConfiguredSecretAcceptsATokenFromTheSameSecret(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)
	token := signedToken(t, testSecret, jwt.MapClaims{
		"role": "anon",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})

	if !verifier.SignedByConfiguredSecret(token) {
		t.Error("SignedByConfiguredSecret = false for a token with no sub, want true")
	}
}

func TestSignedByConfiguredSecretRejectsAnotherSecret(t *testing.T) {
	verifier := supabase.NewVerifier(testSecret)
	token := signedToken(t, "a-different-secret", jwt.MapClaims{
		"role": "anon",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})

	if verifier.SignedByConfiguredSecret(token) {
		t.Error("SignedByConfiguredSecret = true for another secret, want false")
	}
}

func TestSignedByConfiguredSecretRejectsWithoutASecret(t *testing.T) {
	token := signedToken(t, testSecret, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	if supabase.NewVerifier("").SignedByConfiguredSecret(token) {
		t.Error("SignedByConfiguredSecret = true with no configured secret, want false")
	}
}
