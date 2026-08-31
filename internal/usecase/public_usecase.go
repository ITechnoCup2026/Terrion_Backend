package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/plots"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

var ErrPublicPlotNotFound = errors.New("public plot not found")

type PublicUseCase struct {
	DB                    *gorm.DB
	Log                   *logrus.Logger
	PublicPlotRepository  *repository.PublicPlotRepository
	PlotRepository        *repository.PlotRepository
	BlockRepository       *repository.BlockRepository
	CommodityRepository   *repository.CommodityRepository
	VarietyRepository     *repository.VarietyRepository
	CooperativeRepository *repository.CooperativeRepository
	Weather               *WeatherUseCase
}

func NewPublicUseCase(
	db *gorm.DB, log *logrus.Logger,
	publicPlotRepository *repository.PublicPlotRepository,
	plotRepository *repository.PlotRepository,
	blockRepository *repository.BlockRepository,
	commodityRepository *repository.CommodityRepository,
	varietyRepository *repository.VarietyRepository,
	cooperativeRepository *repository.CooperativeRepository,
	weatherUseCase *WeatherUseCase,
) *PublicUseCase {
	return &PublicUseCase{
		DB:                    db,
		Log:                   log,
		PublicPlotRepository:  publicPlotRepository,
		PlotRepository:        plotRepository,
		BlockRepository:       blockRepository,
		CommodityRepository:   commodityRepository,
		VarietyRepository:     varietyRepository,
		CooperativeRepository: cooperativeRepository,
		Weather:               weatherUseCase,
	}
}

type YieldRange struct {
	MinTonnes float64
	MaxTonnes float64
}

type PublicBlock struct {
	ID            string
	Label         string
	AreaHa        float64
	OrderIndex    int
	CommodityName string
	VarietyName   string
	SpriteRow     int
	PlantingDate  time.Time
	Window        *agronomy.HarvestWindow
	YieldRange    *YieldRange
}

type PublicPlot struct {
	View            entity.PublicPlot
	Blocks          []PublicBlock
	Degraded        bool
	CooperativeName *string
	Neighbours      plots.Neighbours
}

func (u *PublicUseCase) LoadPlot(
	ctx context.Context, publicID string, now time.Time,
) (PublicPlot, error) {
	db := u.DB.WithContext(ctx)

	view, err := u.PublicPlotRepository.FindByPublicID(db, publicID)
	if err != nil {
		return PublicPlot{}, fmt.Errorf("reading public plot %s: %w", publicID, err)
	}
	if view == nil {
		return PublicPlot{}, ErrPublicPlotNotFound
	}

	plot, err := u.PlotRepository.FindByPublicID(db, publicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PublicPlot{}, ErrPublicPlotNotFound
		}
		return PublicPlot{}, fmt.Errorf("reading plot %s: %w", publicID, err)
	}

	standing, err := u.standingBlocks(db, plot.ID)
	if err != nil {
		return PublicPlot{}, err
	}

	cell := weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
	cellWeather, err := u.Weather.LoadWeatherFor(ctx, cell, time.Time{}, now)
	if err != nil {
		return PublicPlot{}, err
	}
	degraded := len(cellWeather.Normals) == 0

	blocks, err := u.describeBlocks(db, standing, cellWeather, degraded, now)
	if err != nil {
		return PublicPlot{}, err
	}

	neighbours, cooperativeName, err := u.neighboursOf(db, *view)
	if err != nil {
		return PublicPlot{}, err
	}

	return PublicPlot{
		View:            *view,
		Blocks:          blocks,
		Degraded:        degraded,
		CooperativeName: cooperativeName,
		Neighbours:      neighbours,
	}, nil
}

func (u *PublicUseCase) standingBlocks(db *gorm.DB, plotID string) ([]entity.Block, error) {
	all, err := u.BlockRepository.FindByPlotID(db, plotID)
	if err != nil {
		return nil, fmt.Errorf("reading blocks of plot %s: %w", plotID, err)
	}

	standing := []entity.Block{}
	for _, block := range all {
		if block.ActualHarvestDate == nil {
			standing = append(standing, block)
		}
	}
	return standing, nil
}

func (u *PublicUseCase) describeBlocks(
	db *gorm.DB, blocks []entity.Block, cellWeather CellWeather, degraded bool, now time.Time,
) ([]PublicBlock, error) {
	commodityIDs := map[string]bool{}
	varietyIDs := map[string]bool{}
	for _, block := range blocks {
		commodityIDs[block.CommodityID] = true
		varietyIDs[block.VarietyID] = true
	}

	commodityRows, err := u.CommodityRepository.FindByIDs(db, keysOf(commodityIDs))
	if err != nil {
		return nil, fmt.Errorf("reading commodities: %w", err)
	}
	commodities := make(map[string]entity.Commodity, len(commodityRows))
	for _, row := range commodityRows {
		commodities[row.ID] = row
	}

	varietyRows, err := u.VarietyRepository.FindByIDs(db, keysOf(varietyIDs))
	if err != nil {
		return nil, fmt.Errorf("reading varieties: %w", err)
	}
	varieties := make(map[string]entity.Variety, len(varietyRows))
	for _, row := range varietyRows {
		varieties[row.ID] = row
	}

	today := agronomy.ToISODate(now)
	observed, forecast := splitOnToday(cellWeather.Observed, today)

	described := make([]PublicBlock, len(blocks))
	for i, block := range blocks {
		commodity := commodities[block.CommodityID]
		row, known := varieties[block.VarietyID]

		described[i] = PublicBlock{
			ID:            block.ID,
			Label:         block.Label,
			AreaHa:        block.AreaHa,
			OrderIndex:    block.OrderIndex,
			CommodityName: commodity.Name,
			VarietyName:   row.Name,
			SpriteRow:     commodity.SpriteRow,
			PlantingDate:  block.PlantingDate,
		}
		if !known {
			continue
		}

		described[i].YieldRange = &YieldRange{
			MinTonnes: row.YieldPerHaMin * block.AreaHa,
			MaxTonnes: row.YieldPerHaMax * block.AreaHa,
		}
		if degraded {
			continue
		}

		window, err := agronomy.PredictHarvest(agronomy.HarvestInput{
			PlantingDate: block.PlantingDate,
			Observed:     observed,
			Forecast:     forecast,
			Climatology:  cellWeather.Normals,
			Variety: agronomy.Variety{
				GddRequirement:   row.GddRequirement,
				BaseTempC:        row.BaseTempC,
				DaysToHarvestMin: row.DaysToHarvestMin,
				DaysToHarvestMax: row.DaysToHarvestMax,
				YieldPerHaMin:    row.YieldPerHaMin,
				YieldPerHaMax:    row.YieldPerHaMax,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("projecting block %s: %w", block.ID, err)
		}
		described[i].Window = &window
	}
	return described, nil
}

func (u *PublicUseCase) neighboursOf(
	db *gorm.DB, view entity.PublicPlot,
) (plots.Neighbours, *string, error) {
	siblings, err := u.PublicPlotRepository.FindInVillage(db, view.Village, view.District)
	if err != nil {
		return plots.Neighbours{}, nil,
			fmt.Errorf("reading the plots of %s: %w", view.Village, err)
	}

	list := make([]plots.Neighbour, len(siblings))
	for i, sibling := range siblings {
		list[i] = plots.Neighbour{
			PublicID:   sibling.PublicID,
			Name:       sibling.Name,
			MemberName: sibling.MemberName,
			AreaHa:     sibling.AreaHa,
		}
	}

	cooperative, err := u.CooperativeRepository.FindInVillage(db, view.Village, view.District)
	if err != nil {
		return plots.Neighbours{}, nil,
			fmt.Errorf("reading the cooperative of %s: %w", view.Village, err)
	}

	var name *string
	if cooperative != nil {
		found := cooperative.Name
		name = &found
	}

	return plots.NeighboursOf(list, view.PublicID), name, nil
}
