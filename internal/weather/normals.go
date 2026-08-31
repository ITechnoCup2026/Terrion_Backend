package weather

import (
	"fmt"
	"math"
	"sort"

	"terrion-backend/internal/agronomy"
)

func DeriveNormals(days []agronomy.TempDay) ([]agronomy.ClimateNormal, error) {
	meansByDayOfYear := map[int][]float64{}

	for _, day := range days {
		parsed, err := agronomy.UTCDate(day.Date)
		if err != nil {
			return nil, fmt.Errorf("deriving normals: %w", err)
		}
		dayOfYear := agronomy.DayOfYear(parsed)
		meansByDayOfYear[dayOfYear] = append(meansByDayOfYear[dayOfYear], (day.TMin+day.TMax)/2)
	}

	daysOfYear := make([]int, 0, len(meansByDayOfYear))
	for dayOfYear := range meansByDayOfYear {
		daysOfYear = append(daysOfYear, dayOfYear)
	}
	sort.Ints(daysOfYear)

	normals := make([]agronomy.ClimateNormal, 0, len(daysOfYear))
	for _, dayOfYear := range daysOfYear {
		means := meansByDayOfYear[dayOfYear]
		normals = append(normals, agronomy.ClimateNormal{
			DayOfYear: dayOfYear,
			MeanC:     average(means),
			SdC:       sampleStandardDeviation(means),
		})
	}
	return normals, nil
}

func average(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func sampleStandardDeviation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	mean := average(values)
	sumSquares := 0.0
	for _, value := range values {
		sumSquares += (value - mean) * (value - mean)
	}
	return math.Sqrt(sumSquares / float64(len(values)-1))
}
