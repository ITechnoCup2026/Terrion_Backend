package repository

import (
	"time"

	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type SeasonPlanRepository struct {
	Repository[entity.SeasonPlan]
}

func (r *SeasonPlanRepository) FindActive(
	db *gorm.DB, cooperativeID string, seasonStart time.Time,
) (*entity.SeasonPlan, error) {
	plan := new(entity.SeasonPlan)
	err := db.Where("cooperative_id = ? AND season_start = ? AND status = ?",
		cooperativeID, seasonStart, "applied").Take(plan).Error
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (r *SeasonPlanRepository) FindByCooperativeID(
	db *gorm.DB, cooperativeID string,
) ([]entity.SeasonPlan, error) {
	plans := []entity.SeasonPlan{}
	err := db.Where("cooperative_id = ?", cooperativeID).
		Order("season_start DESC").Find(&plans).Error
	return plans, err
}

func (r *SeasonPlanRepository) Cancel(db *gorm.DB, planID string) error {
	return db.Model(&entity.SeasonPlan{}).
		Where("id = ? AND status = ?", planID, "applied").
		Update("status", "cancelled").Error
}

func (r *SeasonPlanRepository) DeleteBlocksOfPlan(db *gorm.DB, planID string) error {
	return db.Where("season_plan_id = ? AND actual_harvest_date IS NULL", planID).
		Delete(&entity.Block{}).Error
}
