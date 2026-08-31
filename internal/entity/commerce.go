package entity

import (
	"time"

	"terrion-backend/internal/constants"
)

// Rupiah amounts are float64 throughout, matching the arithmetic ported from
// the Next.js implementation. These figures are aggregates over tonnages that
// are themselves estimates, so the rounding error is orders of magnitude below
// the uncertainty already in the number. A ledger would need a decimal type;
// this is not one.
type SupplyContractRequest struct {
	ID            string `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	CooperativeID string `gorm:"column:cooperative_id"`
	BuyerID       string `gorm:"column:buyer_id"`
	// Copied from the buyer's profile when the request is made, not joined.
	// A buyer renaming their organisation must not rewrite what the cooperative
	// already agreed to supply.
	BuyerName         string                  `gorm:"column:buyer_name"`
	BuyerOrganisation *string                 `gorm:"column:buyer_organisation"`
	CommodityID       string                  `gorm:"column:commodity_id"`
	VolumeKg          float64                 `gorm:"column:volume_kg"`
	WindowStart       time.Time               `gorm:"column:window_start"`
	WindowEnd         time.Time               `gorm:"column:window_end"`
	Status            constants.RequestStatus `gorm:"column:status;type:request_status"`
	Notes             *string                 `gorm:"column:notes"`
	CreatedAt         time.Time               `gorm:"column:created_at"`
	RespondedAt       *time.Time              `gorm:"column:responded_at"`
}

func (SupplyContractRequest) TableName() string { return "supply_contract_request" }

type InputOrder struct {
	ID            string                `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	CooperativeID string                `gorm:"column:cooperative_id"`
	SeasonLabel   string                `gorm:"column:season_label"`
	Status        constants.OrderStatus `gorm:"column:status;type:order_status"`
	CreatedAt     time.Time             `gorm:"column:created_at"`
}

func (InputOrder) TableName() string { return "input_order" }

// Prices are null until a supplier quotes them. Impact figure 3 counts only
// completed orders carrying both, so an unquoted order correctly contributes
// nothing rather than a fabricated saving.
type InputOrderLine struct {
	ID                 string   `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	InputOrderID       string   `gorm:"column:input_order_id"`
	Item               string   `gorm:"column:item"`
	Quantity           float64  `gorm:"column:quantity"`
	Unit               string   `gorm:"column:unit"`
	RetailPricePerUnit *float64 `gorm:"column:retail_price_per_unit"`
	BulkPricePerUnit   *float64 `gorm:"column:bulk_price_per_unit"`
}

func (InputOrderLine) TableName() string { return "input_order_line" }
