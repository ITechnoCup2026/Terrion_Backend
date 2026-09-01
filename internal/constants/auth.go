package constants

import "time"

const (
	AuthUserLocal = "auth_user"
	BearerPrefix  = "Bearer "
)

const (
	SessionCookieName = "terrion_session"
	SessionKeyPrefix  = "terrion:session:"
	SessionIDBytes    = 32
	SessionTTL        = 30 * 24 * time.Hour
)

const (
	GoTrueSignUpPath        = "/auth/v1/signup"
	GoTrueAdminUsersPath    = "/auth/v1/admin/users"
	GoTrueTokenPasswordPath = "/auth/v1/token?grant_type=password"
	GoTrueTokenRefreshPath  = "/auth/v1/token?grant_type=refresh_token"
	GoTrueLogoutPath        = "/auth/v1/logout"
	GoTrueTimeout           = OpenMeteoTimeout
)

const (
	SignupOutcomeSignedIn     = "signed_in"
	SignupOutcomeConfirmEmail = "confirm_email"
)

const (
	SignupErrorUnavailable = "internal"
	SignupErrorValidation  = "validation_failed"
)
