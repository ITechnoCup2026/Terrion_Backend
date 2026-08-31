package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type BlockRepository struct {
	Repository[entity.Block]
}

func (r *BlockRepository) FindByPlotIDs(db *gorm.DB, plotIDs []string) ([]entity.Block, error) {
	blocks := []entity.Block{}
	if len(plotIDs) == 0 {
		return blocks, nil
	}

	err := db.Where("plot_id IN ?", plotIDs).Order("plot_id, order_index").
		Find(&blocks).Error
	return blocks, err
}
