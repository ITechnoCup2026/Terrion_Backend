package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type CommodityRepository struct {
	Repository[entity.Commodity]
}

func (r *CommodityRepository) FindAll(db *gorm.DB) ([]entity.Commodity, error) {
	commodities := []entity.Commodity{}
	err := db.Order("sprite_row, name").Find(&commodities).Error
	return commodities, err
}

func (r *CommodityRepository) FindByIDs(db *gorm.DB, ids []string) ([]entity.Commodity, error) {
	commodities := []entity.Commodity{}
	if len(ids) == 0 {
		return commodities, nil
	}

	err := db.Where("id IN ?", ids).Find(&commodities).Error
	return commodities, err
}
