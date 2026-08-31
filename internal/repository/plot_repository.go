package repository

import (
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type PlotRepository struct {
	Repository[entity.Plot]
}

func (r *PlotRepository) FindByCooperativeID(
	db *gorm.DB, cooperativeID string,
) ([]entity.Plot, error) {
	plots := []entity.Plot{}
	err := db.Where("cooperative_id = ?", cooperativeID).Order("created_at, id").
		Find(&plots).Error
	return plots, err
}

func (r *PlotRepository) FindInCooperative(
	db *gorm.DB, plotID, cooperativeID string,
) (*entity.Plot, error) {
	plot := new(entity.Plot)
	err := db.Where("id = ? AND cooperative_id = ?", plotID, cooperativeID).Take(plot).Error
	if err != nil {
		return nil, err
	}
	return plot, nil
}

func (r *PlotRepository) FindByPublicID(db *gorm.DB, publicID string) (*entity.Plot, error) {
	plot := new(entity.Plot)
	err := db.Where("public_id = ?", publicID).Take(plot).Error
	if err != nil {
		return nil, err
	}
	return plot, nil
}

func (r *PlotRepository) CountAndAreaByCooperative(
	db *gorm.DB,
) (map[string]PlotTally, error) {
	rows := []struct {
		CooperativeID string
		PlotCount     int
		Hectares      float64
	}{}

	err := db.Model(&entity.Plot{}).
		Select("cooperative_id, COUNT(*) AS plot_count, COALESCE(SUM(area_ha), 0) AS hectares").
		Group("cooperative_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	tally := make(map[string]PlotTally, len(rows))
	for _, row := range rows {
		tally[row.CooperativeID] = PlotTally{Count: row.PlotCount, Hectares: row.Hectares}
	}
	return tally, nil
}

type PlotTally struct {
	Count    int
	Hectares float64
}
