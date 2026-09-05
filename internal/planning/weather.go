package planning

import (
	"time"

	"terrion-backend/internal/agronomy"
)

func SeriesForWindow(
	observed []agronomy.TempDay,
	climatology []agronomy.ClimateNormal,
	from, to time.Time,
) []agronomy.TempDay {
	if !to.After(from) && !to.Equal(from) {
		return nil
	}

	known := make(map[string]agronomy.TempDay, len(observed))
	for _, day := range observed {
		known[day.Date] = day
	}

	normalByDayOfYear := make(map[int]agronomy.ClimateNormal, len(climatology))
	for _, normal := range climatology {
		normalByDayOfYear[normal.DayOfYear] = normal
	}

	series := make([]agronomy.TempDay, 0, agronomy.DaysBetween(from, to)+1)
	for cursor := agronomy.StartOfDay(from); !cursor.After(to); cursor = agronomy.AddDays(cursor, 1) {
		iso := agronomy.ToISODate(cursor)

		if day, ok := known[iso]; ok {
			series = append(series, day)
			continue
		}

		normal, ok := normalByDayOfYear[agronomy.DayOfYear(cursor)]
		if !ok {
			continue
		}
		series = append(series, agronomy.TempDay{Date: iso, TMin: normal.MeanC, TMax: normal.MeanC})
	}

	return series
}
