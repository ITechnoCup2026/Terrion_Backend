package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type InputOrderRepository struct {
	Repository[entity.InputOrder]
}

func (r *InputOrderRepository) FindByCooperativeID(
	db *gorm.DB, cooperativeID string,
) ([]entity.InputOrder, error) {
	orders := []entity.InputOrder{}
	err := db.Where("cooperative_id = ?", cooperativeID).Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

func (r *InputOrderRepository) FindLinesByOrderIDs(
	db *gorm.DB, orderIDs []string,
) ([]entity.InputOrderLine, error) {
	lines := []entity.InputOrderLine{}
	if len(orderIDs) == 0 {
		return lines, nil
	}

	err := db.Where("input_order_id IN ?", orderIDs).Order("item").Find(&lines).Error
	return lines, err
}
