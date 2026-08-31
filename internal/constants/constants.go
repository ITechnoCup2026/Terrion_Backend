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
)

const MigrationsPath = "db/migrations"

const (
	GeneratedPasswordPrefix = "terrion-"
	GeneratedPasswordBytes  = 6
)
