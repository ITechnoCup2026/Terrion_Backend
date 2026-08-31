package constants

// UserRole mirrors the user_role enum. A kader registers land in the field, a
// pengurus commits the cooperative to orders and contracts, and a buyer holds
// no tenant data at all — which is why a buyer has no cooperative.
type UserRole string

const (
	RoleKader    UserRole = "kader"
	RolePengurus UserRole = "pengurus"
	RoleBuyer    UserRole = "buyer"
)

// RequestStatus mirrors the request_status enum. A cooperative answers a
// request; it does not withdraw one on the buyer's behalf, and it cannot move
// an answered request back to pending.
type RequestStatus string

const (
	RequestPending   RequestStatus = "pending"
	RequestAccepted  RequestStatus = "accepted"
	RequestDeclined  RequestStatus = "declined"
	RequestWithdrawn RequestStatus = "withdrawn"
)

// OrderStatus mirrors the order_status enum. Impact figure 3 counts only
// completed orders: nothing has been saved until the cooperative has taken
// delivery.
type OrderStatus string

const (
	OrderDraft     OrderStatus = "draft"
	OrderSubmitted OrderStatus = "submitted"
	OrderCompleted OrderStatus = "completed"
)
