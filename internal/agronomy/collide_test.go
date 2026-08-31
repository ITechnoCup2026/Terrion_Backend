package agronomy_test

import (
	"math"
	"sort"
	"testing"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

func projectionAt(t *testing.T, id, start, end string, tonnes float64) agronomy.BlockProjection {
	t.Helper()
	return agronomy.BlockProjection{
		BlockID:     id,
		PlotID:      "plot-" + id,
		CommodityID: "jagung",
		Window: agronomy.DateRange{
			Start: mustDate(t, start),
			End:   mustDate(t, end),
		},
		ExpectedTonnes: tonnes,
	}
}

func totalTonnes(weeks []agronomy.WeekBucket) float64 {
	total := 0.0
	for _, week := range weeks {
		total += week.Tonnes
	}
	return total
}

func TestDetectCollisionsSpreadsTonnageAcrossEveryWeekAWindowSpans(t *testing.T) {
	report := agronomy.DetectCollisions(
		[]agronomy.BlockProjection{projectionAt(t, "a", "2026-10-15", "2026-10-21", 70)}, nil)

	if len(report.Weeks) != 2 {
		t.Errorf("len(Weeks) = %d, want 2", len(report.Weeks))
	}
	closeTo(t, "total tonnes", totalTonnes(report.Weeks), 70, 5e-6)
}

func TestDetectCollisionsConservesTotalTonnage(t *testing.T) {
	report := agronomy.DetectCollisions([]agronomy.BlockProjection{
		projectionAt(t, "a", "2026-10-05", "2026-10-12", 40),
		projectionAt(t, "b", "2026-10-19", "2026-10-25", 60),
	}, nil)

	closeTo(t, "total tonnes", totalTonnes(report.Weeks), 100, 5e-6)
}

func TestDetectCollisionsFlagsAWeekAboveStatedCapacity(t *testing.T) {
	report := agronomy.DetectCollisions(
		[]agronomy.BlockProjection{projectionAt(t, "a", "2026-10-12", "2026-10-18", 120)},
		map[string]float64{"jagung": 80})

	if len(report.Flagged) != 1 {
		t.Fatalf("len(Flagged) = %d, want 1", len(report.Flagged))
	}
	if report.Flagged[0].Basis != constants.ThresholdCapacity {
		t.Errorf("Basis = %q, want %q", report.Flagged[0].Basis, constants.ThresholdCapacity)
	}
	if report.Flagged[0].Threshold != 80 {
		t.Errorf("Threshold = %v, want 80", report.Flagged[0].Threshold)
	}
}

func TestDetectCollisionsFallsBackToMedianWhenCapacityIsUnset(t *testing.T) {
	projections := []agronomy.BlockProjection{
		projectionAt(t, "f0", "2026-09-07", "2026-09-07", 10),
		projectionAt(t, "f1", "2026-09-14", "2026-09-14", 10),
		projectionAt(t, "f2", "2026-09-21", "2026-09-21", 10),
		projectionAt(t, "f3", "2026-09-28", "2026-09-28", 10),
		projectionAt(t, "f4", "2026-10-05", "2026-10-05", 10),
		projectionAt(t, "f5", "2026-10-12", "2026-10-12", 10),
		projectionAt(t, "spike", "2026-11-02", "2026-11-02", 100),
	}

	report := agronomy.DetectCollisions(projections, nil)

	flaggedOnMedian := false
	for _, week := range report.Flagged {
		if week.Basis == constants.ThresholdMedian {
			flaggedOnMedian = true
		}
	}
	if !flaggedOnMedian {
		t.Errorf("Flagged = %+v, want a week flagged on the median threshold", report.Flagged)
	}
}

func TestDetectCollisionsFlagsNothingWhenTheSeasonIsEvenlySpread(t *testing.T) {
	projections := []agronomy.BlockProjection{
		projectionAt(t, "e0", "2026-09-01", "2026-09-01", 10),
		projectionAt(t, "e1", "2026-09-04", "2026-09-04", 10),
		projectionAt(t, "e2", "2026-09-07", "2026-09-07", 10),
		projectionAt(t, "e3", "2026-09-10", "2026-09-10", 10),
		projectionAt(t, "e4", "2026-09-13", "2026-09-13", 10),
		projectionAt(t, "e5", "2026-09-16", "2026-09-16", 10),
		projectionAt(t, "e6", "2026-09-19", "2026-09-19", 10),
		projectionAt(t, "e7", "2026-09-22", "2026-09-22", 10),
	}

	if flagged := agronomy.DetectCollisions(projections, nil).Flagged; len(flagged) != 0 {
		t.Errorf("len(Flagged) = %d, want 0", len(flagged))
	}
}

func TestDetectCollisionsNamesContributingBlocks(t *testing.T) {
	report := agronomy.DetectCollisions([]agronomy.BlockProjection{
		projectionAt(t, "a", "2026-10-12", "2026-10-14", 60),
		projectionAt(t, "b", "2026-10-15", "2026-10-16", 60),
	}, map[string]float64{"jagung": 80})

	if len(report.Flagged) != 1 {
		t.Fatalf("len(Flagged) = %d, want 1", len(report.Flagged))
	}
	got := append([]string{}, report.Flagged[0].ContributingBlockIDs...)
	sort.Strings(got)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("ContributingBlockIDs = %v, want [a b]", got)
	}
}

func TestDetectCollisionsSuggestsShiftsThatBringThePeakBelowThreshold(t *testing.T) {
	report := agronomy.DetectCollisions([]agronomy.BlockProjection{
		projectionAt(t, "a", "2026-10-12", "2026-10-14", 60),
		projectionAt(t, "b", "2026-10-15", "2026-10-16", 60),
	}, map[string]float64{"jagung": 80})

	if len(report.Suggestions) == 0 {
		t.Fatal("len(Suggestions) = 0, want at least one")
	}
	suggestion := report.Suggestions[0]

	if suggestion.ResultingTonnes > 80 {
		t.Errorf("ResultingTonnes = %v, want at most 80", suggestion.ResultingTonnes)
	}
	if shift := math.Abs(float64(suggestion.ShiftDays)); shift < 7 || shift > 14 {
		t.Errorf("ShiftDays = %d, want a magnitude between 7 and 14", suggestion.ShiftDays)
	}
}

func TestDetectCollisionsOfNothingIsEmpty(t *testing.T) {
	report := agronomy.DetectCollisions(nil, nil)

	if len(report.Weeks) != 0 || len(report.Flagged) != 0 || len(report.Suggestions) != 0 {
		t.Errorf("report = %+v, want everything empty", report)
	}
}
