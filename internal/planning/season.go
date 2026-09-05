package planning

import (
	"fmt"
	"time"

	"terrion-backend/internal/agronomy"
)

type Season struct {
	Label        string
	Start        time.Time
	End          time.Time
	PlantingFrom time.Time
	PlantingTo   time.Time
}

func utcDay(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func SeasonMT1(startYear int) Season {
	return Season{
		Label:        fmt.Sprintf("MT I %d/%d", startYear, startYear+1),
		Start:        utcDay(startYear, time.October, 1),
		End:          utcDay(startYear+1, time.March, 31),
		PlantingFrom: utcDay(startYear, time.October, 1),
		PlantingTo:   utcDay(startYear, time.December, 31),
	}
}

func SeasonMT2(year int) Season {
	return Season{
		Label:        fmt.Sprintf("MT II %d", year),
		Start:        utcDay(year, time.April, 1),
		End:          utcDay(year, time.September, 30),
		PlantingFrom: utcDay(year, time.April, 1),
		PlantingTo:   utcDay(year, time.June, 30),
	}
}

func OpenSeasons(now time.Time) []Season {
	year := agronomy.StartOfDay(now).Year()

	candidates := []Season{
		SeasonMT1(year - 1), SeasonMT2(year),
		SeasonMT1(year), SeasonMT2(year + 1),
		SeasonMT1(year + 1),
	}

	open := []Season{}
	for _, season := range candidates {
		if season.PlantingTo.After(agronomy.StartOfDay(now)) {
			open = append(open, season)
		}
	}
	return open
}

func SeasonByLabel(label string, now time.Time) (Season, bool) {
	for _, season := range OpenSeasons(now) {
		if season.Label == label {
			return season, true
		}
	}
	return Season{}, false
}

func CandidatePlantingDates(season Season, now time.Time) []time.Time {
	earliest := agronomy.AddDays(agronomy.StartOfDay(now), 1)
	if season.PlantingFrom.After(earliest) {
		earliest = season.PlantingFrom
	}

	cursor := agronomy.ISOWeekStart(earliest)
	if cursor.Before(earliest) {
		cursor = agronomy.AddDays(cursor, 7)
	}

	dates := []time.Time{}
	for !cursor.After(season.PlantingTo) {
		dates = append(dates, cursor)
		cursor = agronomy.AddDays(cursor, 7)
	}
	return dates
}
