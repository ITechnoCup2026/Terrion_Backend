package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/supabase"
)

var ErrUnauthorised = errors.New("unauthorised")

type AuthUseCase struct {
	DB             *gorm.DB
	Log            *logrus.Logger
	Validate       *validator.Validate
	Redis          *redis.Client
	UserRepository *repository.Repository[entity.AppUser]
	Verifier       *supabase.Verifier
	GoTrue         *supabase.Client
}

func NewAuthUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate, cache *redis.Client,
	userRepository *repository.Repository[entity.AppUser],
	verifier *supabase.Verifier, goTrue *supabase.Client,
) *AuthUseCase {
	return &AuthUseCase{
		DB:             db,
		Log:            log,
		Validate:       validate,
		Redis:          cache,
		UserRepository: userRepository,
		Verifier:       verifier,
		GoTrue:         goTrue,
	}
}

func (u *AuthUseCase) Authenticate(ctx context.Context, sessionID string) (*entity.AppUser, error) {
	if sessionID == "" {
		return nil, ErrUnauthorised
	}

	session, found, err := u.readSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("reading session %s: %w", sessionID, err)
	}
	if !found {
		return nil, ErrUnauthorised
	}

	return u.loadProfile(ctx, session.UserID)
}

func (u *AuthUseCase) SignUpBuyer(
	ctx context.Context, request *model.SignupRequest,
) (*model.SignupResponse, error) {
	if err := u.Validate.Struct(request); err != nil {
		return nil, fmt.Errorf("validating signup request: %w", err)
	}

	email := strings.ToLower(strings.TrimSpace(request.Email))

	created, err := u.GoTrue.SignUp(ctx, email, request.Password)
	if err != nil {
		return nil, err
	}

	if created.AlreadyRegistered {
		return &model.SignupResponse{
			Outcome: constants.SignupOutcomeConfirmEmail,
			Email:   email,
		}, nil
	}

	if err := u.createBuyerProfile(ctx, created.UserID, request); err != nil {
		if deleteErr := u.GoTrue.DeleteUser(ctx, created.UserID); deleteErr != nil {
			u.Log.Errorf("orphaned auth user %s after failed profile insert: %v",
				created.UserID, deleteErr)
		}
		return nil, err
	}

	if created.HasSession {
		return &model.SignupResponse{Outcome: constants.SignupOutcomeSignedIn}, nil
	}
	return &model.SignupResponse{
		Outcome: constants.SignupOutcomeConfirmEmail,
		Email:   email,
	}, nil
}

type sessionData struct {
	UserID       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (u *AuthUseCase) sessionKey(sessionID string) string {
	return constants.SessionKeyPrefix + sessionID
}

func newSessionID() (string, error) {
	raw := make([]byte, constants.SessionIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (u *AuthUseCase) readSession(ctx context.Context, sessionID string) (sessionData, bool, error) {
	raw, err := u.Redis.Get(ctx, u.sessionKey(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return sessionData{}, false, nil
	}
	if err != nil {
		return sessionData{}, false, fmt.Errorf("reading session: %w", err)
	}

	var data sessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return sessionData{}, false, fmt.Errorf("decoding session: %w", err)
	}
	return data, true, nil
}

func (u *AuthUseCase) writeSession(ctx context.Context, sessionID string, session supabase.Session) error {
	data := sessionData{
		UserID:       session.UserID,
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encoding session: %w", err)
	}
	if err := u.Redis.Set(ctx, u.sessionKey(sessionID), raw, constants.SessionTTL).Err(); err != nil {
		return fmt.Errorf("writing session: %w", err)
	}
	return nil
}

func (u *AuthUseCase) deleteSession(ctx context.Context, sessionID string) error {
	if err := u.Redis.Del(ctx, u.sessionKey(sessionID)).Err(); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

func (u *AuthUseCase) loadProfile(ctx context.Context, userID string) (*entity.AppUser, error) {
	user := new(entity.AppUser)
	if err := u.UserRepository.FindById(u.DB.WithContext(ctx), user, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUnauthorised
		}
		return nil, fmt.Errorf("loading profile for auth subject %s: %w", userID, err)
	}
	return user, nil
}

func (u *AuthUseCase) Login(
	ctx context.Context, request *model.LoginRequest,
) (string, *entity.AppUser, error) {
	if err := u.Validate.Struct(request); err != nil {
		return "", nil, fmt.Errorf("validating login request: %w", err)
	}

	email := strings.ToLower(strings.TrimSpace(request.Email))

	session, err := u.GoTrue.SignIn(ctx, email, request.Password)
	if err != nil {
		return "", nil, err
	}

	sessionID, err := newSessionID()
	if err != nil {
		return "", nil, err
	}
	if err := u.writeSession(ctx, sessionID, session); err != nil {
		return "", nil, err
	}

	user, err := u.loadProfile(ctx, session.UserID)
	if err != nil {
		return "", nil, err
	}
	return sessionID, user, nil
}

func (u *AuthUseCase) RefreshSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return ErrUnauthorised
	}

	data, found, err := u.readSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("reading session %s: %w", sessionID, err)
	}
	if !found {
		return ErrUnauthorised
	}

	refreshed, err := u.GoTrue.RefreshToken(ctx, data.RefreshToken)
	if err != nil {
		return err
	}

	return u.writeSession(ctx, sessionID, refreshed)
}

func (u *AuthUseCase) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	data, found, err := u.readSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("reading session %s: %w", sessionID, err)
	}
	if found {
		if err := u.GoTrue.Logout(ctx, data.AccessToken); err != nil {
			u.Log.Warnf("revoking session %s at supabase: %v", sessionID, err)
		}
	}

	return u.deleteSession(ctx, sessionID)
}

func (u *AuthUseCase) createBuyerProfile(
	ctx context.Context, userID string, request *model.SignupRequest,
) error {
	organisation := strings.TrimSpace(request.Organisation)

	profile := &entity.AppUser{
		ID:            userID,
		Role:          constants.RoleBuyer,
		CooperativeID: nil,
		FullName:      strings.TrimSpace(request.FullName),
		Organisation:  &organisation,
	}

	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := u.UserRepository.Create(tx, profile); err != nil {
		return fmt.Errorf("creating buyer profile %s: %w", userID, err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing buyer profile %s: %w", userID, err)
	}
	return nil
}
