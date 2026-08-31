package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

type WeatherUseCase struct {
	DB         *gorm.DB
	Log        *logrus.Logger
	Repository *repository.WeatherRepository
	OpenMeteo  *weather.Client
}

func NewWeatherUseCase(
	db *gorm.DB, log *logrus.Logger,
	weatherRepository *repository.WeatherRepository, openMeteo *weather.Client,
) *WeatherUseCase {
	return &WeatherUseCase{
		DB:         db,
		Log:        log,
		Repository: weatherRepository,
		OpenMeteo:  openMeteo,
	}
}

type BackfillResult struct {
	Skipped bool
	Rows    int
}

type RefreshResult struct {
	Cells       int
	RowsWritten int
	Backfilled  int
	Failed      []string
}

type CellWeather struct {
	Observed []agronomy.TempDay
	Normals  []agronomy.ClimateNormal
}

func (u *WeatherUseCase) BackfillForPlot(
	ctx context.Context, lat, lng float64, now time.Time,
) (BackfillResult, error) {
	return u.BackfillGrid(ctx, weather.SnapToGrid(lat, lng), now)
}

func (u *WeatherUseCase) BackfillGrid(
	ctx context.Context, cell weather.GridCell, now time.Time,
) (BackfillResult, error) {
	stored, err := u.Repository.CountDaily(u.DB.WithContext(ctx), cell)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("counting stored weather for %v: %w", cell, err)
	}
	if stored >= constants.WeatherBackfillCompleteRows {
		return BackfillResult{Skipped: true, Rows: int(stored)}, nil
	}

	end := agronomy.StartOfDay(now)
	start := agronomy.AddDays(end, -365*constants.WeatherHistoryYears)

	days, err := u.OpenMeteo.FetchHistory(ctx, cell.GridLat, cell.GridLng, start, end)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("backfilling %v: %w", cell, err)
	}

	normals, err := weather.DeriveNormals(days)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("backfilling %v: %w", cell, err)
	}

	if err := u.write(ctx, cell, days, normals); err != nil {
		return BackfillResult{}, err
	}
	return BackfillResult{Rows: len(days)}, nil
}

func (u *WeatherUseCase) RefreshAllGrids(ctx context.Context, now time.Time) (RefreshResult, error) {
	cells, err := u.Repository.FindOccupiedGridCells(u.DB.WithContext(ctx))
	if err != nil {
		return RefreshResult{}, fmt.Errorf("scanning plots for occupied grid cells: %w", err)
	}

	result := RefreshResult{Cells: len(cells), Failed: []string{}}
	end := agronomy.StartOfDay(now)

	for _, cell := range cells {
		rows, backfilled, err := u.refreshCell(ctx, cell, end, now)
		if err != nil {
			result.Failed = append(result.Failed,
				fmt.Sprintf("%v,%v: %v", cell.GridLat, cell.GridLng, err))
			continue
		}
		result.RowsWritten += rows
		if backfilled {
			result.Backfilled++
		}
	}

	return result, nil
}

func (u *WeatherUseCase) refreshCell(
	ctx context.Context, cell weather.GridCell, end, now time.Time,
) (int, bool, error) {
	backfill, err := u.BackfillGrid(ctx, cell, now)
	if err != nil {
		return 0, false, err
	}
	if !backfill.Skipped {
		return 0, true, nil
	}

	lookbackStart := agronomy.AddDays(end, -constants.WeatherRefreshLookbackDays)
	recent, err := u.OpenMeteo.FetchHistory(ctx, cell.GridLat, cell.GridLng, lookbackStart, end)
	if err != nil {
		return 0, false, err
	}

	forecast, err := u.OpenMeteo.FetchForecast(ctx, cell.GridLat, cell.GridLng)
	if err != nil {
		return 0, false, err
	}

	days := append(append([]agronomy.TempDay{}, forecast...), recent...)
	if err := u.write(ctx, cell, days, nil); err != nil {
		return 0, false, err
	}
	return len(days), false, nil
}

func (u *WeatherUseCase) LoadWeatherFor(
	ctx context.Context, cell weather.GridCell, since, now time.Time,
) (CellWeather, error) {
	if since.IsZero() {
		since = agronomy.AddDays(agronomy.StartOfDay(now), -constants.MaxProjectionDays)
	}

	db := u.DB.WithContext(ctx)

	daily, err := u.Repository.FindDailySince(db, cell, since)
	if err != nil {
		return CellWeather{}, fmt.Errorf("reading stored weather for %v: %w", cell, err)
	}

	stored, err := u.Repository.FindNormals(db, cell)
	if err != nil {
		return CellWeather{}, fmt.Errorf("reading climate normals for %v: %w", cell, err)
	}

	observed := make([]agronomy.TempDay, len(daily))
	for i, row := range daily {
		observed[i] = agronomy.TempDay{
			Date: agronomy.ToISODate(row.Date),
			TMin: row.TempMin,
			TMax: row.TempMax,
		}
	}

	normals := make([]agronomy.ClimateNormal, len(stored))
	for i, row := range stored {
		normals[i] = agronomy.ClimateNormal{
			DayOfYear: row.DayOfYear,
			MeanC:     row.MeanC,
			SdC:       row.SdC,
		}
	}

	return CellWeather{Observed: observed, Normals: normals}, nil
}

func (u *WeatherUseCase) write(
	ctx context.Context, cell weather.GridCell,
	days []agronomy.TempDay, normals []agronomy.ClimateNormal,
) error {
	dailyRows, err := dailyRowsFor(cell, days)
	if err != nil {
		return err
	}

	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := u.Repository.UpsertDaily(tx, dailyRows); err != nil {
		return fmt.Errorf("storing weather for %v: %w", cell, err)
	}
	if err := u.Repository.UpsertNormals(tx, normalRowsFor(cell, normals)); err != nil {
		return fmt.Errorf("storing climate normals for %v: %w", cell, err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing weather for %v: %w", cell, err)
	}
	return nil
}

func dailyRowsFor(cell weather.GridCell, days []agronomy.TempDay) ([]entity.WeatherDaily, error) {
	positionByDate := map[string]int{}
	rows := []entity.WeatherDaily{}

	for _, day := range days {
		date, err := agronomy.UTCDate(day.Date)
		if err != nil {
			return nil, fmt.Errorf("storing weather for %v: %w", cell, err)
		}

		row := entity.WeatherDaily{
			GridLat: cell.GridLat,
			GridLng: cell.GridLng,
			Date:    date,
			TempMin: day.TMin,
			TempMax: day.TMax,
		}

		if position, seen := positionByDate[day.Date]; seen {
			rows[position] = row
			continue
		}
		positionByDate[day.Date] = len(rows)
		rows = append(rows, row)
	}
	return rows, nil
}

func normalRowsFor(cell weather.GridCell, normals []agronomy.ClimateNormal) []entity.WeatherNormal {
	rows := make([]entity.WeatherNormal, len(normals))
	for i, normal := range normals {
		rows[i] = entity.WeatherNormal{
			GridLat:   cell.GridLat,
			GridLng:   cell.GridLng,
			DayOfYear: normal.DayOfYear,
			MeanC:     normal.MeanC,
			SdC:       normal.SdC,
		}
	}
	return rows
}
