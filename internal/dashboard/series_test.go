package dashboard_test

import (
	"math"
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/dashboard"
)

var weekZero = agronomy.ISOWeekStart(time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))

func projection(blockID string, start, end time.Time, tonnes float64) agronomy.BlockProjection {
	return agronomy.BlockProjection{
		BlockID:        blockID,
		PlotID:         "plot-" + blockID,
		CommodityID:    "padi",
		Window:         agronomy.DateRange{Start: start, End: end},
		ExpectedTonnes: tonnes,
	}
}

func closeTo(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestWeeklyProjectionReturnsTheRequestedNumberOfWeeks(t *testing.T) {
	if weeks := dashboard.WeeklyProjection(nil, weekZero, 12); len(weeks) != 12 {
		t.Errorf("len(weeks) = %d, want 12", len(weeks))
	}
}

func TestWeeklyProjectionDefaultsToTwelveWeeks(t *testing.T) {
	if weeks := dashboard.WeeklyProjection(nil, weekZero, 0); len(weeks) != 12 {
		t.Errorf("len(weeks) = %d, want the 12-week default", len(weeks))
	}
}

func TestWeeklyProjectionStartsAtTheWeekContainingFrom(t *testing.T) {
	weeks := dashboard.WeeklyProjection(nil, agronomy.AddDays(weekZero, 3), 3)

	for i, week := range weeks {
		want := agronomy.AddDays(weekZero, i*7)
		if !week.WeekStart.Equal(want) {
			t.Errorf("weeks[%d].WeekStart = %v, want %v", i, week.WeekStart, want)
		}
	}
	if weeks[0].ISOWeek != agronomy.ISOWeekKey(weekZero) {
		t.Errorf("ISOWeek = %q, want %q", weeks[0].ISOWeek, agronomy.ISOWeekKey(weekZero))
	}
}

func TestWeeklyProjectionKeepsEmptyWeeksAtZero(t *testing.T) {
	weeks := dashboard.WeeklyProjection([]agronomy.BlockProjection{
		projection("a", weekZero, agronomy.AddDays(weekZero, 6), 10),
	}, weekZero, 3)

	quiet := weeks[1]
	if quiet.ExpectedTonnes != 0 || quiet.MinTonnes != 0 || quiet.MaxTonnes != 0 {
		t.Errorf("quiet week = %+v, want all zero", quiet)
	}
	if len(quiet.BlockIDs) != 0 {
		t.Errorf("BlockIDs = %v, want empty", quiet.BlockIDs)
	}
}

func TestWeeklyProjectionCreditsAWindowInsideOneWeekEntirely(t *testing.T) {
	weeks := dashboard.WeeklyProjection([]agronomy.BlockProjection{
		projection("a", agronomy.AddDays(weekZero, 1), agronomy.AddDays(weekZero, 5), 14),
	}, weekZero, 2)

	closeTo(t, "ExpectedTonnes", weeks[0].ExpectedTonnes, 14)
	closeTo(t, "MinTonnes", weeks[0].MinTonnes, 14)
	closeTo(t, "MaxTonnes", weeks[0].MaxTonnes, 14)
	if len(weeks[0].BlockIDs) != 1 || weeks[0].BlockIDs[0] != "a" {
		t.Errorf("BlockIDs = %v, want [a]", weeks[0].BlockIDs)
	}
}

func TestWeeklyProjectionSplitsAStraddlingWindowProportionally(t *testing.T) {
	weeks := dashboard.WeeklyProjection([]agronomy.BlockProjection{
		projection("a", agronomy.AddDays(weekZero, 1), agronomy.AddDays(weekZero, 8), 80),
	}, weekZero, 2)

	closeTo(t, "week 0 ExpectedTonnes", weeks[0].ExpectedTonnes, 60)
	closeTo(t, "week 1 ExpectedTonnes", weeks[1].ExpectedTonnes, 20)
}

func TestWeeklyProjectionGivesAStraddlingWindowNoMinimumAndFullMaximum(t *testing.T) {
	weeks := dashboard.WeeklyProjection([]agronomy.BlockProjection{
		projection("a", agronomy.AddDays(weekZero, 1), agronomy.AddDays(weekZero, 8), 80),
	}, weekZero, 2)

	for i, week := range weeks {
		if week.MinTonnes != 0 {
			t.Errorf("weeks[%d].MinTonnes = %v, want 0", i, week.MinTonnes)
		}
		closeTo(t, "MaxTonnes", week.MaxTonnes, 80)
	}
}

func TestWeeklyProjectionBracketsTheExpectedValue(t *testing.T) {
	weeks := dashboard.WeeklyProjection([]agronomy.BlockProjection{
		projection("a", agronomy.AddDays(weekZero, 1), agronomy.AddDays(weekZero, 8), 80),
		projection("b", agronomy.AddDays(weekZero, 2), agronomy.AddDays(weekZero, 4), 5),
	}, weekZero, 2)

	for i, week := range weeks {
		if week.MinTonnes > week.ExpectedTonnes+1e-9 {
			t.Errorf("weeks[%d]: min %v exceeds expected %v", i, week.MinTonnes, week.ExpectedTonnes)
		}
		if week.MaxTonnes < week.ExpectedTonnes-1e-9 {
			t.Errorf("weeks[%d]: max %v below expected %v", i, week.MaxTonnes, week.ExpectedTonnes)
		}
	}
}

func TestWeeklyProjectionIgnoresProjectionsOutsideTheHorizon(t *testing.T) {
	weeks := dashboard.WeeklyProjection([]agronomy.BlockProjection{
		projection("old", agronomy.AddDays(weekZero, -30), agronomy.AddDays(weekZero, -25), 99),
	}, weekZero, 4)

	for i, week := range weeks {
		if week.ExpectedTonnes != 0 {
			t.Errorf("weeks[%d].ExpectedTonnes = %v, want 0", i, week.ExpectedTonnes)
		}
	}
}
