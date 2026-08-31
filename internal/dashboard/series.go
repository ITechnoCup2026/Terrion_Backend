package dashboard

import (
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

type ProjectionWeek struct {
	ISOWeek        string
	WeekStart      time.Time
	ExpectedTonnes float64
	MinTonnes      float64
	MaxTonnes      float64
	BlockIDs       []string
}

func WeeklyProjection(
	projections []agronomy.BlockProjection, from time.Time, weeks int,
) []ProjectionWeek {
	if weeks <= 0 {
		weeks = constants.DefaultHorizonWeeks
	}
	first := agronomy.ISOWeekStart(from)

	horizon := make([]ProjectionWeek, weeks)
	for i := range horizon {
		weekStart := agronomy.AddDays(first, i*7)
		weekEnd := agronomy.AddDays(weekStart, 6)

		week := ProjectionWeek{
			ISOWeek:   agronomy.ISOWeekKey(weekStart),
			WeekStart: weekStart,
			BlockIDs:  []string{},
		}

		for _, projection := range projections {
			span := max(1, agronomy.DaysBetween(projection.Window.Start, projection.Window.End)+1)
			inside := overlapDays(projection.Window.Start, projection.Window.End, weekStart, weekEnd)
			if inside == 0 {
				continue
			}

			week.ExpectedTonnes += projection.ExpectedTonnes / float64(span) * float64(inside)
			week.MaxTonnes += projection.ExpectedTonnes
			if !projection.Window.Start.Before(weekStart) && !projection.Window.End.After(weekEnd) {
				week.MinTonnes += projection.ExpectedTonnes
			}
			week.BlockIDs = append(week.BlockIDs, projection.BlockID)
		}

		horizon[i] = week
	}
	return horizon
}

func overlapDays(start, end, weekStart, weekEnd time.Time) int {
	from := start
	if weekStart.After(from) {
		from = weekStart
	}
	to := end
	if weekEnd.Before(to) {
		to = weekEnd
	}

	days := agronomy.DaysBetween(from, to) + 1
	if days < 0 {
		return 0
	}
	return days
}
