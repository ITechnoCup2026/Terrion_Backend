package constants

type UserRole string

const (
	RoleKader    UserRole = "kader"
	RolePengurus UserRole = "pengurus"
	RoleBuyer    UserRole = "buyer"
)

type RequestStatus string

const (
	RequestPending   RequestStatus = "pending"
	RequestAccepted  RequestStatus = "accepted"
	RequestDeclined  RequestStatus = "declined"
	RequestWithdrawn RequestStatus = "withdrawn"
)

type OrderStatus string

const (
	OrderDraft     OrderStatus = "draft"
	OrderSubmitted OrderStatus = "submitted"
	OrderCompleted OrderStatus = "completed"
	OrderCancelled OrderStatus = "cancelled"
)

const (
	OrderNotFound          = "order_not_found"
	OrderTransitionInvalid = "order_transition_invalid"
	OrderAlreadyFinal      = "order_already_final"
	OrderSeasonAlreadyOpen = "order_season_already_open"
	OrderLineUnknown       = "order_line_unknown"
	OrderLinesEmpty        = "order_lines_empty"
)

const MigrationsPath = "db/migrations"

const (
	GeneratedPasswordPrefix = "terrion-"
	GeneratedPasswordBytes  = 6
)

const (
	EnvFileName    = ".env"
	ModuleFileName = "go.mod"
)
