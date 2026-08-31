package supabase

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

var ErrNoJWTSecret = errors.New("SUPABASE_JWT_SECRET is not configured")

type Verifier struct {
	secret []byte
}

func NewVerifier(secret string) *Verifier {
	return &Verifier{secret: []byte(secret)}
}

func (v *Verifier) Subject(token string) (string, error) {
	if len(v.secret) == 0 {
		return "", ErrNoJWTSecret
	}

	parsed, err := jwt.Parse(token,
		func(*jwt.Token) (any, error) { return v.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", fmt.Errorf("verifying access token: %w", err)
	}

	subject, err := parsed.Claims.GetSubject()
	if err != nil {
		return "", fmt.Errorf("reading access token subject: %w", err)
	}
	if subject == "" {
		return "", errors.New("access token carries no subject")
	}

	return subject, nil
}

func (v *Verifier) SignedByConfiguredSecret(token string) bool {
	if len(v.secret) == 0 {
		return false
	}

	_, err := jwt.Parse(token,
		func(*jwt.Token) (any, error) { return v.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	return err == nil
}
