package agronomy

import (
	"fmt"
	"math"
	"sort"
	"time"

	"terrion-backend/internal/constants"
)

type HarvestInput struct {
	PlantingDate time.Time
	Observed     []TempDay
	Forecast     []TempDay
	Climatology  []ClimateNormal
	Variety      Variety
	Calibration  *Calibration
}

func ShrunkOffset(calibration Calibration) float64 {
	observations := float64(calibration.NObservations)
	return calibration.OffsetDays * (observations / (observations + constants.CalibrationShrinkageK))
}

func IsImplausible(window HarvestWindow) bool {
	return window.Plausibility == constants.PlausibilityImplausible
}

func PredictHarvest(input HarvestInput) (HarvestWindow, error) {
	plantedISO := ToISODate(input.PlantingDate)
	known := mergeWeatherSincePlanting(input.Forecast, input.Observed, plantedISO)

	cumulativeGdd := AccumulateGdd(known, input.Variety.BaseTempC)
	gddAccumulated := 0.0
	if len(cumulativeGdd) > 0 {
		gddAccumulated = cumulativeGdd[len(cumulativeGdd)-1].Gdd
	}

	projectionStart, err := firstUnknownDay(known, input.PlantingDate)
	if err != nil {
		return HarvestWindow{}, err
	}

	normalByDayOfYear := make(map[int]ClimateNormal, len(input.Climatology))
	for _, normal := range input.Climatology {
		normalByDayOfYear[normal.DayOfYear] = normal
	}

	maturity := maturitySearch{
		plantingDate:    input.PlantingDate,
		projectionStart: projectionStart,
		accumulated:     gddAccumulated,
		required:        input.Variety.GddRequirement,
		variety:         input.Variety,
		normals:         normalByDayOfYear,
	}

	earlyDap := float64(maturity.daysAfterPlanting(constants.ZEarly))
	lateDap := float64(maturity.daysAfterPlanting(constants.ZLate))

	if input.Calibration != nil {
		shift := ShrunkOffset(*input.Calibration)
		earlyDap += shift - input.Calibration.ResidualSd
		lateDap += shift + input.Calibration.ResidualSd
	}

	startDap := math.Round(math.Min(earlyDap, lateDap))
	endDap := math.Round(math.Max(earlyDap, lateDap))

	return HarvestWindow{
		Start:          AddDays(input.PlantingDate, int(startDap)),
		End:            AddDays(input.PlantingDate, int(endDap)),
		Confidence:     constants.HarvestWindowConfidence,
		GddAccumulated: gddAccumulated,
		GddRequired:    input.Variety.GddRequirement,
		Stage:          GrowthStageFor(gddAccumulated, input.Variety.GddRequirement),
		Basis:          basisOf(input, plantedISO),
		Plausibility:   judgePlausibility((startDap+endDap)/2, input.Variety),
		CumulativeGdd:  cumulativeGdd,
	}, nil
}

func mergeWeatherSincePlanting(forecast, observed []TempDay, plantedISO string) []TempDay {
	byDate := make(map[string]TempDay, len(forecast)+len(observed))
	for _, day := range forecast {
		if day.Date >= plantedISO {
			byDate[day.Date] = day
		}
	}
	for _, day := range observed {
		if day.Date >= plantedISO {
			byDate[day.Date] = day
		}
	}

	merged := make([]TempDay, 0, len(byDate))
	for _, day := range byDate {
		merged = append(merged, day)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Date < merged[j].Date })
	return merged
}

func firstUnknownDay(known []TempDay, plantingDate time.Time) (time.Time, error) {
	if len(known) == 0 {
		return plantingDate, nil
	}

	last := known[len(known)-1].Date
	parsed, err := UTCDate(last)
	if err != nil {
		return time.Time{}, fmt.Errorf("last known weather day: %w", err)
	}
	return AddDays(parsed, 1), nil
}

type maturitySearch struct {
	plantingDate    time.Time
	projectionStart time.Time
	accumulated     float64
	required        float64
	variety         Variety
	normals         map[int]ClimateNormal
}

func (search maturitySearch) daysAfterPlanting(z float64) int {
	total := search.accumulated
	cursor := search.projectionStart

	if total >= search.required {
		return max(0, DaysBetween(search.plantingDate, cursor)-1)
	}

	for range constants.MaxProjectionDays {
		meanC := search.variety.BaseTempC
		if normal, ok := search.normals[DayOfYear(cursor)]; ok {
			meanC = normal.MeanC + z*normal.SdC
		}

		total += GddForDay(TempDay{TMin: meanC, TMax: meanC}, search.variety.BaseTempC)
		if total >= search.required {
			return DaysBetween(search.plantingDate, cursor)
		}
		cursor = AddDays(cursor, 1)
	}

	return constants.MaxProjectionDays
}

func basisOf(input HarvestInput, plantedISO string) constants.WindowBasis {
	if coversPlanting(input.Observed, plantedISO) {
		return constants.BasisObserved
	}
	if coversPlanting(input.Forecast, plantedISO) {
		return constants.BasisForecast
	}
	return constants.BasisClimatology
}

func coversPlanting(days []TempDay, plantedISO string) bool {
	for _, day := range days {
		if day.Date >= plantedISO {
			return true
		}
	}
	return false
}

func judgePlausibility(midDap float64, variety Variety) constants.Plausibility {
	floor := float64(variety.DaysToHarvestMin)
	ceiling := float64(variety.DaysToHarvestMax)

	switch {
	case midDap >= floor && midDap <= ceiling:
		return constants.PlausibilityOk
	case midDap < floor:
		if midDap >= floor*(1-constants.VarietyBoundsTolerance) {
			return constants.PlausibilityEarly
		}
		return constants.PlausibilityImplausible
	case midDap <= ceiling*(1+constants.VarietyBoundsTolerance):
		return constants.PlausibilityLate
	default:
		return constants.PlausibilityImplausible
	}
}
