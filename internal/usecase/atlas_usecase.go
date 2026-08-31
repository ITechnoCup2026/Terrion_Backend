package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
)

var ErrCooperativeNotFound = errors.New("cooperative not found")

type AtlasUseCase struct {
	DB                    *gorm.DB
	Log                   *logrus.Logger
	CooperativeRepository *repository.CooperativeRepository
	PlotRepository        *repository.PlotRepository
	PublicPlotRepository  *repository.PublicPlotRepository
	BlockRepository       *repository.BlockRepository
	CommodityRepository   *repository.CommodityRepository
}

func NewAtlasUseCase(
	db *gorm.DB, log *logrus.Logger,
	cooperativeRepository *repository.CooperativeRepository,
	plotRepository *repository.PlotRepository,
	publicPlotRepository *repository.PublicPlotRepository,
	blockRepository *repository.BlockRepository,
	commodityRepository *repository.CommodityRepository,
) *AtlasUseCase {
	return &AtlasUseCase{
		DB:                    db,
		Log:                   log,
		CooperativeRepository: cooperativeRepository,
		PlotRepository:        plotRepository,
		PublicPlotRepository:  publicPlotRepository,
		BlockRepository:       blockRepository,
		CommodityRepository:   commodityRepository,
	}
}

type AtlasCooperative struct {
	ID        string
	Name      string
	Village   string
	District  string
	Province  string
	Lat       float64
	Lng       float64
	PlotCount int
	Hectares  float64
}

type AtlasPlot struct {
	PublicID   string
	Name       string
	MemberName string
	AreaHa     float64
	Crops      []string
}

type AtlasFarm struct {
	CooperativeID string
	Name          string
	Village       string
	District      string
	Province      string
	Plots         []AtlasPlot
	TotalHectares float64
}

func (u *AtlasUseCase) Cooperatives(ctx context.Context) ([]AtlasCooperative, error) {
	db := u.DB.WithContext(ctx)

	rows, err := u.CooperativeRepository.FindAll(db)
	if err != nil {
		return nil, fmt.Errorf("reading cooperatives: %w", err)
	}

	tally, err := u.PlotRepository.CountAndAreaByCooperative(db)
	if err != nil {
		return nil, fmt.Errorf("counting plots per cooperative: %w", err)
	}

	cooperatives := make([]AtlasCooperative, len(rows))
	for i, row := range rows {
		cooperatives[i] = AtlasCooperative{
			ID:        row.ID,
			Name:      row.Name,
			Village:   row.Village,
			District:  row.District,
			Province:  row.Province,
			Lat:       row.Lat,
			Lng:       row.Lng,
			PlotCount: tally[row.ID].Count,
			Hectares:  tally[row.ID].Hectares,
		}
	}
	return cooperatives, nil
}

func (u *AtlasUseCase) Farm(ctx context.Context, cooperativeID string) (AtlasFarm, error) {
	db := u.DB.WithContext(ctx)

	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(db, cooperative, cooperativeID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AtlasFarm{}, ErrCooperativeNotFound
		}
		return AtlasFarm{}, fmt.Errorf("reading cooperative %s: %w", cooperativeID, err)
	}

	views, err := u.PublicPlotRepository.FindInVillage(
		db, cooperative.Village, cooperative.District)
	if err != nil {
		return AtlasFarm{}, fmt.Errorf("reading the plots of %s: %w", cooperative.Village, err)
	}

	farm := AtlasFarm{
		CooperativeID: cooperative.ID,
		Name:          cooperative.Name,
		Village:       cooperative.Village,
		District:      cooperative.District,
		Province:      cooperative.Province,
		Plots:         make([]AtlasPlot, len(views)),
	}

	publicIDs := make([]string, len(views))
	for i, view := range views {
		publicIDs[i] = view.PublicID
		farm.Plots[i] = AtlasPlot{
			PublicID:   view.PublicID,
			Name:       view.Name,
			MemberName: view.MemberName,
			AreaHa:     view.AreaHa,
			Crops:      []string{},
		}
		farm.TotalHectares += view.AreaHa
	}

	crops, err := u.standingCrops(db, publicIDs)
	if err != nil {
		return AtlasFarm{}, err
	}
	for i, plot := range farm.Plots {
		if names, growing := crops[plot.PublicID]; growing {
			farm.Plots[i].Crops = names
		}
	}

	return farm, nil
}

func (u *AtlasUseCase) standingCrops(
	db *gorm.DB, publicIDs []string,
) (map[string][]string, error) {
	crops := map[string][]string{}
	if len(publicIDs) == 0 {
		return crops, nil
	}

	rows := []struct {
		PublicID    string
		CommodityID string
	}{}

	err := db.Model(&entity.Block{}).
		Select("plot.public_id AS public_id, block.commodity_id AS commodity_id").
		Joins("JOIN plot ON plot.id = block.plot_id").
		Where("plot.public_id IN ? AND block.actual_harvest_date IS NULL", publicIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("reading standing crops: %w", err)
	}

	commodityIDs := map[string]bool{}
	for _, row := range rows {
		commodityIDs[row.CommodityID] = true
	}

	commodities, err := u.CommodityRepository.FindByIDs(db, keysOf(commodityIDs))
	if err != nil {
		return nil, fmt.Errorf("reading commodities: %w", err)
	}
	nameOf := make(map[string]string, len(commodities))
	for _, commodity := range commodities {
		nameOf[commodity.ID] = commodity.Name
	}

	seen := map[string]bool{}
	for _, row := range rows {
		name, known := nameOf[row.CommodityID]
		if !known || seen[row.PublicID+"|"+name] {
			continue
		}
		seen[row.PublicID+"|"+name] = true
		crops[row.PublicID] = append(crops[row.PublicID], name)
	}
	return crops, nil
}
