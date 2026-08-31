package entity

import (
	"time"

	"terrion-backend/internal/constants"
)

type SupplyContractRequest struct {
	ID                string                  `gorm:"column:id;primaryKey"`
	CooperativeID     string                  `gorm:"column:cooperative_id"`
	BuyerID           string                  `gorm:"column:buyer_id"`
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
	ID            string                `gorm:"column:id;primaryKey"`
	CooperativeID string                `gorm:"column:cooperative_id"`
	SeasonLabel   string                `gorm:"column:season_label"`
	Status        constants.OrderStatus `gorm:"column:status;type:order_status"`
	CreatedAt     time.Time             `gorm:"column:created_at"`
}

func (InputOrder) TableName() string { return "input_order" }

type InputOrderLine struct {
	ID                 string   `gorm:"column:id;primaryKey"`
	InputOrderID       string   `gorm:"column:input_order_id"`
	Item               string   `gorm:"column:item"`
	Quantity           float64  `gorm:"column:quantity"`
	Unit               string   `gorm:"column:unit"`
	RetailPricePerUnit *float64 `gorm:"column:retail_price_per_unit"`
	BulkPricePerUnit   *float64 `gorm:"column:bulk_price_per_unit"`
}

func (InputOrderLine) TableName() string { return "input_order_line" }
