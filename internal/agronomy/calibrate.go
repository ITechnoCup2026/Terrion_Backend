package agronomy

import (
	"math"
	"time"
)

type CalibrationObservation struct {
	PredictedMid time.Time
	Actual       time.Time
}

func FitCalibration(observations []CalibrationObservation) Calibration {
	count := len(observations)
	if count == 0 {
		return Calibration{}
	}

	errors := make([]float64, count)
	for i, observation := range observations {
		errors[i] = float64(DaysBetween(observation.PredictedMid, observation.Actual))
	}

	total := 0.0
	for _, days := range errors {
		total += days
	}
	mean := total / float64(count)

	variance := 0.0
	if count > 1 {
		sumSquares := 0.0
		for _, days := range errors {
			sumSquares += (days - mean) * (days - mean)
		}
		variance = sumSquares / float64(count-1)
	}

	return Calibration{
		OffsetDays:    mean,
		NObservations: count,
		ResidualSd:    math.Sqrt(variance),
	}
}
