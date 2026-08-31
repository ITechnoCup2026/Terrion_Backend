package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type SupplyRequestRepository struct {
	Repository[entity.SupplyContractRequest]
}

func (r *SupplyRequestRepository) FindForCooperative(
	db *gorm.DB, cooperativeID string,
) ([]entity.SupplyContractRequest, error) {
	requests := []entity.SupplyContractRequest{}
	err := db.Where("cooperative_id = ?", cooperativeID).Order("created_at DESC").
		Find(&requests).Error
	return requests, err
}

func (r *SupplyRequestRepository) FindForBuyer(
	db *gorm.DB, buyerID string,
) ([]entity.SupplyContractRequest, error) {
	requests := []entity.SupplyContractRequest{}
	err := db.Where("buyer_id = ?", buyerID).Order("created_at DESC").Find(&requests).Error
	return requests, err
}
