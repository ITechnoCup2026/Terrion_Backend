package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type ReferencePriceRepository struct {
	Repository[entity.ReferencePrice]
}

func (r *ReferencePriceRepository) FindForCommodities(
	db *gorm.DB, province string, commodityIDs []string,
) ([]entity.ReferencePrice, error) {
	prices := []entity.ReferencePrice{}
	if province == "" || len(commodityIDs) == 0 {
		return prices, nil
	}

	err := db.Where("province = ? AND commodity_id IN ?", province, commodityIDs).
		Find(&prices).Error
	return prices, err
}
