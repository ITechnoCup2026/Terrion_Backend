package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type PlotRepository struct {
	Repository[entity.Plot]
}

func (r *PlotRepository) FindByCooperativeID(
	db *gorm.DB, cooperativeID string,
) ([]entity.Plot, error) {
	plots := []entity.Plot{}
	err := db.Where("cooperative_id = ?", cooperativeID).Order("created_at, id").
		Find(&plots).Error
	return plots, err
}
