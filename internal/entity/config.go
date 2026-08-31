package entity

import "time"

// CooperativeCapacity is what a cooperative says it can handle in a week. When
// it is absent the collision detector falls back to a multiple of the median
// week, so an unset capacity degrades to a heuristic rather than to no alerts.
type CooperativeCapacity struct {
	CooperativeID string  `gorm:"column:cooperative_id;primaryKey"`
	CommodityID   string  `gorm:"column:commodity_id;primaryKey"`
	TonnesPerWeek float64 `gorm:"column:tonnes_per_week"`
}

func (CooperativeCapacity) TableName() string { return "cooperative_capacity" }

// Calibration is how wrong the harvest model has been for this cooperative and
// variety, fitted from past harvests. NObservations is what shrinks the
// correction toward zero when it rests on only a handful of them.
type Calibration struct {
	CooperativeID string    `gorm:"column:cooperative_id;primaryKey"`
	VarietyID     string    `gorm:"column:variety_id;primaryKey"`
	OffsetDays    float64   `gorm:"column:offset_days"`
	NObservations int       `gorm:"column:n_observations"`
	ResidualSd    float64   `gorm:"column:residual_sd"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (Calibration) TableName() string { return "calibration" }
