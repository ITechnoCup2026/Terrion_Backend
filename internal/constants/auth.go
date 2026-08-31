package constants

const (
	AuthUserLocal = "auth_user"
	BearerPrefix  = "Bearer "
)

const (
	GoTrueSignUpPath     = "/auth/v1/signup"
	GoTrueAdminUsersPath = "/auth/v1/admin/users"
	GoTrueTimeout        = OpenMeteoTimeout
)

const (
	SignupOutcomeSignedIn     = "signed_in"
	SignupOutcomeConfirmEmail = "confirm_email"
)

const (
	SignupErrorUnavailable = "internal"
	SignupErrorValidation  = "validation_failed"
)
