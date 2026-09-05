package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
)

type SeasonPlanRepository struct {
	Repository[entity.SeasonPlan]
}

func (r *SeasonPlanRepository) FindActiveByLabel(
	db *gorm.DB, cooperativeID, seasonLabel string,
) (*entity.SeasonPlan, error) {
	plan := new(entity.SeasonPlan)
	err := db.Where("cooperative_id = ? AND season_label = ? AND status = ?",
		cooperativeID, seasonLabel, constants.PlanApplied).Take(plan).Error
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (r *SeasonPlanRepository) FindInCooperative(
	db *gorm.DB, planID, cooperativeID string,
) (*entity.SeasonPlan, error) {
	plan := new(entity.SeasonPlan)
	err := db.Where("id = ? AND cooperative_id = ?", planID, cooperativeID).Take(plan).Error
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
		Order("created_at DESC, id").Find(&plans).Error
	return plans, err
}

func (r *SeasonPlanRepository) FindItemsByPlanID(
	db *gorm.DB, planID string,
) ([]entity.SeasonPlanItem, error) {
	items := []entity.SeasonPlanItem{}
	err := db.Where("plan_id = ?", planID).
		Order("planting_date, plot_id").Find(&items).Error
	return items, err
}
