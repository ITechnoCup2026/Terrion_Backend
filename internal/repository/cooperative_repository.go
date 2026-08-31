package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type CooperativeRepository struct {
	Repository[entity.Cooperative]
}

func (r *CooperativeRepository) FindCapacity(
	db *gorm.DB, cooperativeID string,
) ([]entity.CooperativeCapacity, error) {
	capacity := []entity.CooperativeCapacity{}
	err := db.Where("cooperative_id = ?", cooperativeID).Find(&capacity).Error
	return capacity, err
}

func (r *CooperativeRepository) FindAll(db *gorm.DB) ([]entity.Cooperative, error) {
	cooperatives := []entity.Cooperative{}
	err := db.Order("name").Find(&cooperatives).Error
	return cooperatives, err
}
