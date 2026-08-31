package repository

import (
	"errors"

	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type PublicPlotRepository struct {
	Repository[entity.PublicPlot]
}

func (r *PublicPlotRepository) FindByPublicID(
	db *gorm.DB, publicID string,
) (*entity.PublicPlot, error) {
	plot := new(entity.PublicPlot)
	err := db.Where("public_id = ?", publicID).Take(plot).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return plot, nil
}

func (r *PublicPlotRepository) FindInVillage(
	db *gorm.DB, village, district string,
) ([]entity.PublicPlot, error) {
	plots := []entity.PublicPlot{}
	err := db.Where("village = ? AND district = ?", village, district).
		Order("name").Find(&plots).Error
	return plots, err
}
