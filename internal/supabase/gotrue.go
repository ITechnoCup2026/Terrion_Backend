package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"terrion-backend/internal/constants"
)

type Client struct {
	HTTP           *http.Client
	URL            string
	AnonKey        string
	ServiceRoleKey string
}

func NewClient(url, anonKey, serviceRoleKey string) *Client {
	return &Client{
		HTTP:           &http.Client{Timeout: constants.GoTrueTimeout},
		URL:            strings.TrimSuffix(url, "/"),
		AnonKey:        anonKey,
		ServiceRoleKey: serviceRoleKey,
	}
}

type AuthError struct {
	Code   string
	Status int
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("gotrue rejected the request with status %d (%s)", e.Status, e.Code)
}

type SignUpResult struct {
	UserID            string
	HasSession        bool
	AlreadyRegistered bool
}

type identity struct {
	ID string `json:"id"`
}

type signUpUser struct {
	ID         string      `json:"id"`
	Identities *[]identity `json:"identities"`
}

type signUpResponse struct {
	signUpUser
	AccessToken string      `json:"access_token"`
	User        *signUpUser `json:"user"`
}

func (c *Client) SignUp(ctx context.Context, email, password string) (SignUpResult, error) {
	body, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		return SignUpResult{}, fmt.Errorf("building signup request: %w", err)
	}

	response, err := c.send(ctx, http.MethodPost,
		c.URL+constants.GoTrueSignUpPath, c.AnonKey, bytes.NewReader(body))
	if err != nil {
		return SignUpResult{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return SignUpResult{}, authErrorFrom(response)
	}

	var decoded signUpResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return SignUpResult{}, fmt.Errorf("decoding signup response: %w", err)
	}

	user := decoded.signUpUser
	if decoded.User != nil {
		user = *decoded.User
	}
	if user.ID == "" {
		return SignUpResult{}, fmt.Errorf("signup response carries no user")
	}

	return SignUpResult{
		UserID:            user.ID,
		HasSession:        decoded.AccessToken != "",
		AlreadyRegistered: user.Identities != nil && len(*user.Identities) == 0,
	}, nil
}

func (c *Client) CreateUser(ctx context.Context, email, password string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"email":         email,
		"password":      password,
		"email_confirm": true,
	})
	if err != nil {
		return "", fmt.Errorf("building create user request: %w", err)
	}

	response, err := c.send(ctx, http.MethodPost,
		c.URL+constants.GoTrueAdminUsersPath, c.ServiceRoleKey, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return "", authErrorFrom(response)
	}

	var created signUpUser
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("decoding create user response: %w", err)
	}
	if created.ID == "" {
		return "", fmt.Errorf("create user response carries no user")
	}
	return created.ID, nil
}

func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	response, err := c.send(ctx, http.MethodDelete,
		c.URL+constants.GoTrueAdminUsersPath+"/"+userID, c.ServiceRoleKey, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return authErrorFrom(response)
	}
	return nil
}

func (c *Client) send(
	ctx context.Context, method, address, key string, body io.Reader,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return nil, fmt.Errorf("building gotrue request: %w", err)
	}
	request.Header.Set("apikey", key)
	request.Header.Set("Authorization", constants.BearerPrefix+key)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("calling gotrue: %w", err)
	}
	return response, nil
}

type gotrueError struct {
	ErrorCode string          `json:"error_code"`
	Code      json.RawMessage `json:"code"`
	Error     string          `json:"error"`
}

func authErrorFrom(response *http.Response) *AuthError {
	var decoded gotrueError
	_ = json.NewDecoder(response.Body).Decode(&decoded)

	return &AuthError{Code: errorCodeOf(decoded), Status: response.StatusCode}
}

func errorCodeOf(decoded gotrueError) string {
	if decoded.ErrorCode != "" {
		return decoded.ErrorCode
	}

	var code string
	if err := json.Unmarshal(decoded.Code, &code); err == nil && code != "" {
		return code
	}
	if decoded.Error != "" {
		return decoded.Error
	}
	return constants.SignupErrorUnavailable
}
