package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// Writes a freshly fitted calibration over whatever was there.
//
// Upsert rather than insert-or-update, because a refit is triggered by
// recording a harvest and two kaders may record one for the same variety at
// the same moment. The table is keyed on (cooperative_id, variety_id), so the
// conflict is exactly the row being rewritten.
//
// A refit reads every recorded harvest for the variety, so the later write
// wins with strictly more information rather than clobbering anything.
func (r *CalibrationRepository) Upsert(db *gorm.DB, calibration *entity.Calibration) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cooperative_id"}, {Name: "variety_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"offset_days", "n_observations", "residual_sd", "updated_at",
		}),
	}).Create(calibration).Error
}
