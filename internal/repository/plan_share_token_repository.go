package repository

import (
	"time"

	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type PlanShareTokenRepository struct {
	Repository[entity.PlanShareToken]
}

func (r *PlanShareTokenRepository) FindByPlanID(
	db *gorm.DB, planID string,
) ([]entity.PlanShareToken, error) {
	tokens := []entity.PlanShareToken{}
	err := db.Where("plan_id = ?", planID).Find(&tokens).Error
	return tokens, err
}

func (r *PlanShareTokenRepository) MarkViewed(
	db *gorm.DB, token string, now time.Time,
) error {
	return db.Model(&entity.PlanShareToken{}).
		Where("id = ?", token).
		Updates(map[string]any{
			"first_viewed_at": gorm.Expr("COALESCE(first_viewed_at, ?)", now),
			"last_viewed_at":  now,
		}).Error
}
