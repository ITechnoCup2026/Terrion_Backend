package entity

import (
	"encoding/json"
	"time"
)

type Plot struct {
	ID            string  `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	CooperativeID string  `gorm:"column:cooperative_id"`
	MemberID      string  `gorm:"column:member_id"`
	PublicID      string  `gorm:"column:public_id"`
	Name          string  `gorm:"column:name"`
	AreaHa        float64 `gorm:"column:area_ha"`
	Lat           float64 `gorm:"column:lat"`
	Lng           float64 `gorm:"column:lng"`

	GridLat    float64 `gorm:"column:grid_lat;->"`
	GridLng    float64 `gorm:"column:grid_lng;->"`
	TileSizeM2 int     `gorm:"column:tile_size_m2;->"`

	TerrainSeed     int             `gorm:"column:terrain_seed"`
	TerrainOverride json.RawMessage `gorm:"column:terrain_override;type:jsonb"`
	Decorations     json.RawMessage `gorm:"column:decorations;type:jsonb"`

	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Plot) TableName() string { return "plot" }

type Block struct {
	ID                  string     `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	PlotID              string     `gorm:"column:plot_id"`
	Label               string     `gorm:"column:label"`
	AreaHa              float64    `gorm:"column:area_ha"`
	OrderIndex          int        `gorm:"column:order_index"`
	CommodityID         string     `gorm:"column:commodity_id"`
	VarietyID           string     `gorm:"column:variety_id"`
	PlantingDate        time.Time  `gorm:"column:planting_date"`
	ActualHarvestDate   *time.Time `gorm:"column:actual_harvest_date"`
	ActualYieldKg       *float64   `gorm:"column:actual_yield_kg"`
	ActualPricePerKg    *float64   `gorm:"column:actual_price_per_kg"`
	PaymentReceivedDate *time.Time `gorm:"column:payment_received_date"`
}

func (Block) TableName() string { return "block" }

type PublicPlot struct {
	PublicID    string  `gorm:"column:public_id"`
	Name        string  `gorm:"column:name"`
	AreaHa      float64 `gorm:"column:area_ha"`
	TileSizeM2  int     `gorm:"column:tile_size_m2"`
	MemberName  string  `gorm:"column:member_name"`
	Village     string  `gorm:"column:village"`
	District    string  `gorm:"column:district"`
	TerrainSeed int     `gorm:"column:terrain_seed"`
}

func (PublicPlot) TableName() string { return "public_plot" }
