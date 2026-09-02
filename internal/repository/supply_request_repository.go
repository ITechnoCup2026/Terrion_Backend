package repository

import (
	"time"

	"gorm.io/gorm"

	"terrion-backend/internal/constants"
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

// SumAcceptedVolumeKg totals every other request already accepted against the
// same cooperative, commodity and harvest window -- what an accept of
// excludeID would be added on top of, for the allocation cap.
func (r *SupplyRequestRepository) SumAcceptedVolumeKg(
	db *gorm.DB, cooperativeID, commodityID string, windowStart, windowEnd time.Time, excludeID string,
) (float64, error) {
	var total float64
	err := db.Model(&entity.SupplyContractRequest{}).
		Where(`cooperative_id = ? AND commodity_id = ? AND window_start = ? AND window_end = ?
			AND status = ? AND id <> ?`,
			cooperativeID, commodityID, windowStart, windowEnd, constants.RequestAccepted, excludeID).
		Select("COALESCE(SUM(volume_kg), 0)").Scan(&total).Error
	return total, err
}
