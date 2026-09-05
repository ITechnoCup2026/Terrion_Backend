package planning

import (
	"time"

	"terrion-backend/internal/agronomy"
)

func SyntheticWeather(
	normals []agronomy.ClimateNormal, from, to time.Time,
) []agronomy.TempDay {
	meanByDayOfYear := make(map[int]float64, len(normals))
	for _, normal := range normals {
		meanByDayOfYear[normal.DayOfYear] = normal.MeanC
	}

	last := agronomy.StartOfDay(to)
	days := []agronomy.TempDay{}

	for cursor := agronomy.StartOfDay(from); !cursor.After(last); cursor = agronomy.AddDays(cursor, 1) {
		meanC, known := meanByDayOfYear[agronomy.DayOfYear(cursor)]
		if !known {
			continue
		}
		days = append(days, agronomy.TempDay{
			Date: agronomy.ToISODate(cursor),
			TMin: meanC,
			TMax: meanC,
		})
	}
	return days
}
