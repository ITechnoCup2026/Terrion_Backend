package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

type ProjectionUseCase struct {
	DB                    *gorm.DB
	Log                   *logrus.Logger
	PlotRepository        *repository.PlotRepository
	BlockRepository       *repository.BlockRepository
	VarietyRepository     *repository.VarietyRepository
	CalibrationRepository *repository.CalibrationRepository
	Weather               *WeatherUseCase
}

func NewProjectionUseCase(
	db *gorm.DB, log *logrus.Logger,
	plotRepository *repository.PlotRepository,
	blockRepository *repository.BlockRepository,
	varietyRepository *repository.VarietyRepository,
	calibrationRepository *repository.CalibrationRepository,
	weatherUseCase *WeatherUseCase,
) *ProjectionUseCase {
	return &ProjectionUseCase{
		DB:                    db,
		Log:                   log,
		PlotRepository:        plotRepository,
		BlockRepository:       blockRepository,
		VarietyRepository:     varietyRepository,
		CalibrationRepository: calibrationRepository,
		Weather:               weatherUseCase,
	}
}

type Projection struct {
	Plots       []entity.Plot
	Blocks      []entity.Block
	Projections []agronomy.BlockProjection
	Windows     map[string]agronomy.HarvestWindow
	Yield       agronomy.YieldModel
}

func (u *ProjectionUseCase) ProjectCooperative(
	ctx context.Context, cooperativeID string, now time.Time,
) (Projection, error) {
	db := u.DB.WithContext(ctx)

	plots, err := u.PlotRepository.FindByCooperativeID(db, cooperativeID)
	if err != nil {
		return Projection{}, fmt.Errorf("reading plots of cooperative %s: %w", cooperativeID, err)
	}
	empty := Projection{
		Plots:   plots,
		Windows: map[string]agronomy.HarvestWindow{},
		Yield:   agronomy.FitYieldModel(nil),
	}
	if len(plots) == 0 {
		return empty, nil
	}

	blocks, err := u.BlockRepository.FindByPlotIDs(db, plotIDsOf(plots))
	if err != nil {
		return Projection{}, fmt.Errorf("reading blocks of cooperative %s: %w", cooperativeID, err)
	}
	empty.Blocks = blocks
	if len(blocks) == 0 {
		return empty, nil
	}

	varieties, err := u.varietiesFor(db, blocks)
	if err != nil {
		return Projection{}, err
	}

	calibrations, err := u.calibrationsFor(db, cooperativeID)
	if err != nil {
		return Projection{}, err
	}

	weatherByCell, err := u.weatherFor(ctx, plots, earliestPlanting(blocks), now)
	if err != nil {
		return Projection{}, err
	}

	cellOfPlot := map[string]weather.GridCell{}
	for _, plot := range plots {
		cellOfPlot[plot.ID] = weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
	}

	weatherOf := func(plotID string) (CellWeather, bool) {
		cell, known := cellOfPlot[plotID]
		if !known {
			return CellWeather{}, false
		}
		cellWeather, stored := weatherByCell[cell]
		return cellWeather, stored
	}

	model := agronomy.FitYieldModel(harvestObservations(blocks, varieties, weatherOf))

	today := agronomy.ToISODate(now)
	projections := []agronomy.BlockProjection{}
	windows := map[string]agronomy.HarvestWindow{}

	for _, block := range blocks {
		if block.ActualHarvestDate != nil {
			continue
		}
		variety, known := varieties[block.VarietyID]
		if !known {
			continue
		}
		cellWeather, stored := weatherOf(block.PlotID)
		if !stored {
			continue
		}

		observed, forecast := splitOnToday(cellWeather.Observed, today)

		window, err := agronomy.PredictHarvest(agronomy.HarvestInput{
			PlantingDate: block.PlantingDate,
			Observed:     observed,
			Forecast:     forecast,
			Climatology:  cellWeather.Normals,
			Variety:      variety,
			Calibration:  calibrations[block.VarietyID],
		})
		if err != nil {
			return Projection{}, fmt.Errorf("projecting block %s: %w", block.ID, err)
		}

		yieldPerHa := agronomy.PredictYieldPerHa(model, agronomy.DeriveYieldFeatures(
			agronomy.YieldFeaturesInput{
				PlantingDate: block.PlantingDate,
				ThroughDate:  now,
				AreaHa:       block.AreaHa,
				Variety:      variety,
				Weather:      observed,
			}))

		windows[block.ID] = window
		projections = append(projections, agronomy.BlockProjection{
			BlockID:        block.ID,
			PlotID:         block.PlotID,
			CommodityID:    block.CommodityID,
			Window:         agronomy.DateRange{Start: window.Start, End: window.End},
			ExpectedTonnes: yieldPerHa * block.AreaHa,
		})
	}

	return Projection{
		Plots:       plots,
		Blocks:      blocks,
		Projections: projections,
		Windows:     windows,
		Yield:       model,
	}, nil
}

func (u *ProjectionUseCase) varietiesFor(
	db *gorm.DB, blocks []entity.Block,
) (map[string]agronomy.Variety, error) {
	ids := map[string]bool{}
	for _, block := range blocks {
		ids[block.VarietyID] = true
	}

	rows, err := u.VarietyRepository.FindByIDs(db, keysOf(ids))
	if err != nil {
		return nil, fmt.Errorf("reading varieties: %w", err)
	}

	varieties := make(map[string]agronomy.Variety, len(rows))
	for _, row := range rows {
		varieties[row.ID] = agronomy.Variety{
			GddRequirement:   row.GddRequirement,
			BaseTempC:        row.BaseTempC,
			DaysToHarvestMin: row.DaysToHarvestMin,
			DaysToHarvestMax: row.DaysToHarvestMax,
			YieldPerHaMin:    row.YieldPerHaMin,
			YieldPerHaMax:    row.YieldPerHaMax,
		}
	}
	return varieties, nil
}

func (u *ProjectionUseCase) calibrationsFor(
	db *gorm.DB, cooperativeID string,
) (map[string]*agronomy.Calibration, error) {
	rows, err := u.CalibrationRepository.FindByCooperativeID(db, cooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading calibrations of cooperative %s: %w", cooperativeID, err)
	}

	calibrations := make(map[string]*agronomy.Calibration, len(rows))
	for _, row := range rows {
		calibrations[row.VarietyID] = &agronomy.Calibration{
			OffsetDays:    row.OffsetDays,
			NObservations: row.NObservations,
			ResidualSd:    row.ResidualSd,
		}
	}
	return calibrations, nil
}

func (u *ProjectionUseCase) weatherFor(
	ctx context.Context, plots []entity.Plot, since, now time.Time,
) (map[weather.GridCell]CellWeather, error) {
	byCell := map[weather.GridCell]CellWeather{}

	for _, plot := range plots {
		cell := weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
		if _, loaded := byCell[cell]; loaded {
			continue
		}

		cellWeather, err := u.Weather.LoadWeatherFor(ctx, cell, since, now)
		if err != nil {
			return nil, err
		}
		byCell[cell] = cellWeather
	}
	return byCell, nil
}

func harvestObservations(
	blocks []entity.Block,
	varieties map[string]agronomy.Variety,
	weatherOf func(plotID string) (CellWeather, bool),
) []agronomy.YieldObservation {
	observations := []agronomy.YieldObservation{}

	for _, block := range blocks {
		if block.ActualHarvestDate == nil || block.ActualYieldKg == nil {
			continue
		}
		variety, known := varieties[block.VarietyID]
		if !known {
			continue
		}
		cellWeather, stored := weatherOf(block.PlotID)
		if !stored {
			continue
		}

		observation, usable := agronomy.DeriveYieldObservation(agronomy.YieldObservationInput{
			PlantingDate:  block.PlantingDate,
			HarvestDate:   *block.ActualHarvestDate,
			AreaHa:        block.AreaHa,
			ActualYieldKg: *block.ActualYieldKg,
			Variety:       variety,
			Weather:       cellWeather.Observed,
		})
		if usable {
			observations = append(observations, observation)
		}
	}
	return observations
}

func splitOnToday(days []agronomy.TempDay, today string) ([]agronomy.TempDay, []agronomy.TempDay) {
	observed := []agronomy.TempDay{}
	forecast := []agronomy.TempDay{}

	for _, day := range days {
		if day.Date <= today {
			observed = append(observed, day)
			continue
		}
		forecast = append(forecast, day)
	}
	return observed, forecast
}

func earliestPlanting(blocks []entity.Block) time.Time {
	earliest := blocks[0].PlantingDate
	for _, block := range blocks {
		if block.PlantingDate.Before(earliest) {
			earliest = block.PlantingDate
		}
	}
	return earliest
}

func plotIDsOf(plots []entity.Plot) []string {
	ids := make([]string, len(plots))
	for i, plot := range plots {
		ids[i] = plot.ID
	}
	return ids
}

func keysOf(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	return keys
}

// RefitCalibration re-derives one variety's day offset from every harvest this
// cooperative has recorded for it, and stores the result.
//
// The comparison is against the BASE model -- PredictHarvest is called with no
// calibration -- because the offset has to mean "how far the uncalibrated
// prediction was out". Feeding the current calibration back in would measure
// the residual of an already-corrected prediction, and drive the offset toward
// zero on every refit until the model had learned nothing at all.
//
// Nothing is stored per prediction. The window is recomputed from the planting
// date, the variety and the weather that actually fell, all of which are still
// on record -- so there is no predicted_mid column to keep in step with the
// predictor, and a stored prediction cannot silently disagree with the model
// that made it.
//
// Returns nil when the variety has no recorded harvest with usable weather.
// There is nothing to say then, and writing a zero offset would claim the model
// had been checked and found exactly right.
func (u *ProjectionUseCase) RefitCalibration(
	ctx context.Context, cooperativeID, varietyID string, now time.Time,
) (*entity.Calibration, error) {
	db := u.DB.WithContext(ctx)

	plots, err := u.PlotRepository.FindByCooperativeID(db, cooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading plots of cooperative %s: %w", cooperativeID, err)
	}
	if len(plots) == 0 {
		return nil, nil
	}

	harvested, err := u.BlockRepository.FindHarvestedByPlotIDs(db, plotIDsOf(plots))
	if err != nil {
		return nil, fmt.Errorf("reading harvested blocks of %s: %w", cooperativeID, err)
	}

	blocks := []entity.Block{}
	for _, block := range harvested {
		if block.VarietyID == varietyID && block.ActualHarvestDate != nil {
			blocks = append(blocks, block)
		}
	}
	if len(blocks) == 0 {
		return nil, nil
	}

	varieties, err := u.varietiesFor(db, blocks)
	if err != nil {
		return nil, err
	}
	variety, known := varieties[varietyID]
	if !known {
		return nil, nil
	}

	weatherByCell, err := u.weatherFor(ctx, plots, earliestPlanting(blocks), now)
	if err != nil {
		return nil, err
	}

	cellOfPlot := map[string]weather.GridCell{}
	for _, plot := range plots {
		cellOfPlot[plot.ID] = weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
	}

	today := agronomy.ToISODate(now)
	observations := []agronomy.CalibrationObservation{}

	for _, block := range blocks {
		cell, placed := cellOfPlot[block.PlotID]
		if !placed {
			continue
		}
		cellWeather, stored := weatherByCell[cell]
		if !stored {
			continue
		}

		observed, forecast := splitOnToday(cellWeather.Observed, today)

		window, err := agronomy.PredictHarvest(agronomy.HarvestInput{
			PlantingDate: block.PlantingDate,
			Observed:     observed,
			Forecast:     forecast,
			Climatology:  cellWeather.Normals,
			Variety:      variety,
			Calibration:  nil,
		})
		if err != nil {
			return nil, fmt.Errorf("re-predicting block %s: %w", block.ID, err)
		}

		observations = append(observations, agronomy.CalibrationObservation{
			PredictedMid: window.Start.Add(window.End.Sub(window.Start) / 2),
			Actual:       *block.ActualHarvestDate,
		})
	}

	if len(observations) == 0 {
		return nil, nil
	}

	fitted := agronomy.FitCalibration(observations)
	row := &entity.Calibration{
		CooperativeID: cooperativeID,
		VarietyID:     varietyID,
		OffsetDays:    fitted.OffsetDays,
		NObservations: fitted.NObservations,
		ResidualSd:    fitted.ResidualSd,
		UpdatedAt:     now,
	}

	if err := u.CalibrationRepository.Upsert(db, row); err != nil {
		return nil, fmt.Errorf("storing calibration of variety %s: %w", varietyID, err)
	}
	return row, nil
}
