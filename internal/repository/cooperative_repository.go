package repository

import (
	"errors"

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

func (r *CooperativeRepository) FindInVillage(
	db *gorm.DB, village, district string,
) (*entity.Cooperative, error) {
	cooperative := new(entity.Cooperative)
	err := db.Where("village = ? AND district = ?", village, district).
		Order("name").Take(cooperative).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cooperative, nil
}
