package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type CalibrationRepository struct {
	Repository[entity.Calibration]
}

func (r *CalibrationRepository) FindByCooperativeID(
	db *gorm.DB, cooperativeID string,
) ([]entity.Calibration, error) {
	calibrations := []entity.Calibration{}
	err := db.Where("cooperative_id = ?", cooperativeID).Find(&calibrations).Error
	return calibrations, err
}
