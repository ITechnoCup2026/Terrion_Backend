package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
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
	UserRepository *repository.Repository[entity.AppUser]
	Verifier       *supabase.Verifier
	GoTrue         *supabase.Client
}

func NewAuthUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate,
	userRepository *repository.Repository[entity.AppUser],
	verifier *supabase.Verifier, goTrue *supabase.Client,
) *AuthUseCase {
	return &AuthUseCase{
		DB:             db,
		Log:            log,
		Validate:       validate,
		UserRepository: userRepository,
		Verifier:       verifier,
		GoTrue:         goTrue,
	}
}

func (u *AuthUseCase) Authenticate(ctx context.Context, token string) (*entity.AppUser, error) {
	if token == "" {
		return nil, ErrUnauthorised
	}

	subject, err := u.Verifier.Subject(token)
	if err != nil {
		u.Log.Debugf("rejecting access token: %v", err)
		return nil, ErrUnauthorised
	}

	user := new(entity.AppUser)
	if err := u.UserRepository.FindById(u.DB.WithContext(ctx), user, subject); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			u.Log.Debugf("no app_user profile for auth subject %s", subject)
			return nil, ErrUnauthorised
		}
		return nil, fmt.Errorf("loading profile for auth subject %s: %w", subject, err)
	}

	return user, nil
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
