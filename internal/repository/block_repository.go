package repository

import (
	"time"

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

func (r *BlockRepository) FindByPlotID(db *gorm.DB, plotID string) ([]entity.Block, error) {
	blocks := []entity.Block{}
	err := db.Where("plot_id = ?", plotID).Order("order_index").Find(&blocks).Error
	return blocks, err
}

func (r *BlockRepository) FindInCooperative(
	db *gorm.DB, blockID, cooperativeID string,
) (*entity.Block, error) {
	block := new(entity.Block)
	err := db.Joins("JOIN plot ON plot.id = block.plot_id").
		Where("block.id = ? AND plot.cooperative_id = ?", blockID, cooperativeID).
		Take(block).Error
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (r *BlockRepository) NextOrderIndex(db *gorm.DB, plotID string) (int, error) {
	var highest *int
	err := db.Model(&entity.Block{}).Where("plot_id = ?", plotID).
		Select("MAX(order_index)").Scan(&highest).Error
	if err != nil {
		return 0, err
	}
	if highest == nil {
		return 0, nil
	}
	return *highest + 1, nil
}

func (r *BlockRepository) FindHarvestedByPlotIDs(
	db *gorm.DB, plotIDs []string,
) ([]entity.Block, error) {
	blocks := []entity.Block{}
	if len(plotIDs) == 0 {
		return blocks, nil
	}

	err := db.Where("plot_id IN ? AND actual_harvest_date IS NOT NULL", plotIDs).
		Find(&blocks).Error
	return blocks, err
}

func (r *BlockRepository) FindPlantedInSeason(
	db *gorm.DB, plotIDs []string, from, to time.Time,
) ([]entity.Block, error) {
	blocks := []entity.Block{}
	if len(plotIDs) == 0 {
		return blocks, nil
	}

	err := db.Where("plot_id IN ? AND planting_date >= ? AND planting_date <= ?",
		plotIDs, from, to).
		Order("plot_id, order_index").Find(&blocks).Error
	return blocks, err
}

func (r *BlockRepository) FindByPlanID(db *gorm.DB, planID string) ([]entity.Block, error) {
	blocks := []entity.Block{}
	err := db.Where("season_plan_id = ?", planID).
		Order("plot_id, order_index").Find(&blocks).Error
	return blocks, err
}
