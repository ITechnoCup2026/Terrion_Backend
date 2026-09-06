package entity

import (
	"time"

	"terrion-backend/internal/constants"
)

type SeasonPlan struct {
	ID            string                      `gorm:"column:id;primaryKey"`
	CooperativeID string                      `gorm:"column:cooperative_id"`
	SeasonLabel   string                      `gorm:"column:season_label"`
	SeasonStart   time.Time                   `gorm:"column:season_start"`
	SeasonEnd     time.Time                   `gorm:"column:season_end"`
	Objective     constants.PlanningObjective `gorm:"column:objective;type:planning_objective"`
	Status        constants.PlanStatus        `gorm:"column:status;type:plan_status"`
	CreatedBy     string                      `gorm:"column:created_by"`
	CreatedAt     time.Time                   `gorm:"column:created_at"`
	CancelledAt   *time.Time                  `gorm:"column:cancelled_at"`
}

func (SeasonPlan) TableName() string { return "season_plan" }

type SeasonPlanItem struct {
	ID                   string    `gorm:"column:id;primaryKey"`
	PlanID               string    `gorm:"column:plan_id"`
	PlotID               string    `gorm:"column:plot_id"`
	MemberID             string    `gorm:"column:member_id"`
	CommodityID          string    `gorm:"column:commodity_id"`
	VarietyID            string    `gorm:"column:variety_id"`
	PlantingDate         time.Time `gorm:"column:planting_date"`
	AreaHa               float64   `gorm:"column:area_ha"`
	ExpectedTonnesLow    float64   `gorm:"column:expected_tonnes_low"`
	ExpectedTonnesMid    float64   `gorm:"column:expected_tonnes_mid"`
	ExpectedTonnesHigh   float64   `gorm:"column:expected_tonnes_high"`
	ExpectedHarvestStart time.Time `gorm:"column:expected_harvest_start"`
	ExpectedHarvestEnd   time.Time `gorm:"column:expected_harvest_end"`
	Plausibility         string    `gorm:"column:plausibility"`
	BlockID              *string   `gorm:"column:block_id"`
}

func (SeasonPlanItem) TableName() string { return "season_plan_item" }

type PlanShareToken struct {
	ID            string     `gorm:"column:id;primaryKey"`
	PlanID        string     `gorm:"column:plan_id;uniqueIndex:plan_share_token_plan_member_idx"`
	MemberID      string     `gorm:"column:member_id;uniqueIndex:plan_share_token_plan_member_idx"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	FirstViewedAt *time.Time `gorm:"column:first_viewed_at"`
	LastViewedAt  *time.Time `gorm:"column:last_viewed_at"`
}

func (PlanShareToken) TableName() string { return "plan_share_token" }
