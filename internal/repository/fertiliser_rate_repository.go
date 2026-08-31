package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type FertiliserRateRepository struct {
	Repository[entity.FertiliserRate]
}

func (r *FertiliserRateRepository) FindAll(db *gorm.DB) ([]entity.FertiliserRate, error) {
	rates := []entity.FertiliserRate{}
	err := db.Order("commodity_id, input_item").Find(&rates).Error
	return rates, err
}
