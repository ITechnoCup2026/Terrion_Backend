package agronomy

import (
	"math"

	"terrion-backend/internal/constants"
)

func GddForDay(day TempDay, baseTempC float64) float64 {
	return math.Max(0, (day.TMax+day.TMin)/2-baseTempC)
}

func AccumulateGdd(days []TempDay, baseTempC float64) []CumulativeGdd {
	total := 0.0
	series := make([]CumulativeGdd, 0, len(days))
	for _, day := range days {
		total += GddForDay(day, baseTempC)
		series = append(series, CumulativeGdd{Date: day.Date, Gdd: total})
	}
	return series
}

func GrowthStageFor(accumulated, required float64) constants.GrowthStage {
	if required <= 0 {
		return constants.StageBare
	}

	switch fraction := accumulated / required; {
	case fraction >= 1:
		return constants.StageReady
	case fraction >= constants.RipeningGddFraction:
		return constants.StageRipening
	case fraction >= constants.VegetativeGddFraction:
		return constants.StageVegetative
	case fraction >= constants.EstablishedGddFraction:
		return constants.StageEstablished
	default:
		return constants.StageBare
	}
}
