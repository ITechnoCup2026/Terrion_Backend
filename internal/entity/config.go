package entity

import "time"

type CooperativeCapacity struct {
	CooperativeID string  `gorm:"column:cooperative_id;primaryKey"`
	CommodityID   string  `gorm:"column:commodity_id;primaryKey"`
	TonnesPerWeek float64 `gorm:"column:tonnes_per_week"`
}

func (CooperativeCapacity) TableName() string { return "cooperative_capacity" }

type Calibration struct {
	CooperativeID string    `gorm:"column:cooperative_id;primaryKey"`
	VarietyID     string    `gorm:"column:variety_id;primaryKey"`
	OffsetDays    float64   `gorm:"column:offset_days"`
	NObservations int       `gorm:"column:n_observations"`
	ResidualSd    float64   `gorm:"column:residual_sd"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (Calibration) TableName() string { return "calibration" }
