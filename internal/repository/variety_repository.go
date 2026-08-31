package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type VarietyRepository struct {
	Repository[entity.Variety]
}

func (r *VarietyRepository) FindByIDs(db *gorm.DB, ids []string) ([]entity.Variety, error) {
	varieties := []entity.Variety{}
	if len(ids) == 0 {
		return varieties, nil
	}

	err := db.Where("id IN ?", ids).Find(&varieties).Error
	return varieties, err
}

func (r *VarietyRepository) FindAll(db *gorm.DB) ([]entity.Variety, error) {
	varieties := []entity.Variety{}
	err := db.Order("name").Find(&varieties).Error
	return varieties, err
}
