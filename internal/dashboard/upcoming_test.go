package dashboard_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/dashboard"
)

var (
	periodFrom = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	periodTo   = time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	ujang      = "Pak Ujang"

	knownPlots = map[string]dashboard.PlotRef{
		"p1": {Name: "Sawah Kidul", MemberName: &ujang},
		"p2": {Name: "Kebun Cabe"},
	}
	knownCommodities = map[string]string{"padi": "Padi"}
)

func window(t *testing.T, blockID, plotID, start, end string, tonnes float64) agronomy.BlockProjection {
	t.Helper()

	from, err := agronomy.UTCDate(start)
	if err != nil {
		t.Fatalf("UTCDate(%q): %v", start, err)
	}
	to, err := agronomy.UTCDate(end)
	if err != nil {
		t.Fatalf("UTCDate(%q): %v", end, err)
	}

	return agronomy.BlockProjection{
		BlockID:        blockID,
		PlotID:         plotID,
		CommodityID:    "padi",
		Window:         agronomy.DateRange{Start: from, End: to},
		ExpectedTonnes: tonnes,
	}
}

func upcoming(projections []agronomy.BlockProjection, limit int) []dashboard.UpcomingHarvest {
	return dashboard.UpcomingHarvests(
		projections, periodFrom, periodTo, knownPlots, knownCommodities, limit)
}

func TestUpcomingHarvestsKeepsEveryWindowOverlappingThePeriod(t *testing.T) {
	tests := []struct {
		name  string
		start string
		end   string
		want  int
	}{
		{"starts inside", "2026-09-03", "2026-09-06", 1},
		{"opened before and still open", "2026-08-28", "2026-09-02", 1},
		{"starts inside and closes after", "2026-09-07", "2026-09-20", 1},
		{"closed before the period", "2026-08-01", "2026-08-20", 0},
		{"opens after the period", "2026-10-01", "2026-10-10", 0},
	}

	for _, test := range tests {
		rows := upcoming([]agronomy.BlockProjection{
			window(t, "b", "p1", test.start, test.end, 1),
		}, 0)

		if len(rows) != test.want {
			t.Errorf("%s: len(rows) = %d, want %d", test.name, len(rows), test.want)
		}
	}
}

func TestUpcomingHarvestsDropsAProjectionWhosePlotIsNotVisible(t *testing.T) {
	rows := upcoming([]agronomy.BlockProjection{
		window(t, "b", "ghost", "2026-09-03", "2026-09-06", 1),
	}, 0)

	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0: a plot the reader may not see must not appear",
			len(rows))
	}
}

func TestUpcomingHarvestsNamesThePlotFarmerAndCommodity(t *testing.T) {
	rows := upcoming([]agronomy.BlockProjection{
		window(t, "b", "p1", "2026-09-03", "2026-09-06", 2.5),
	}, 0)

	row := rows[0]
	if row.PlotName != "Sawah Kidul" || row.CommodityName != "Padi" || row.Tonnes != 2.5 {
		t.Errorf("row = %+v, want Sawah Kidul / Padi / 2.5", row)
	}
	if row.MemberName == nil || *row.MemberName != ujang {
		t.Errorf("MemberName = %v, want %q", row.MemberName, ujang)
	}
}

func TestUpcomingHarvestsSurvivesAPlotWithNoFarmerRecorded(t *testing.T) {
	rows := upcoming([]agronomy.BlockProjection{
		window(t, "b", "p2", "2026-09-03", "2026-09-06", 1),
	}, 0)

	if rows[0].MemberName != nil {
		t.Errorf("MemberName = %v, want nil", *rows[0].MemberName)
	}
}

func TestUpcomingHarvestsSortsSoonestFirst(t *testing.T) {
	rows := upcoming([]agronomy.BlockProjection{
		window(t, "late", "p1", "2026-09-06", "2026-09-07", 1),
		window(t, "early", "p1", "2026-09-02", "2026-09-03", 1),
	}, 0)

	if rows[0].BlockID != "early" || rows[1].BlockID != "late" {
		t.Errorf("order = %q, %q, want early then late", rows[0].BlockID, rows[1].BlockID)
	}
}

func TestUpcomingHarvestsAppliesALimit(t *testing.T) {
	rows := upcoming([]agronomy.BlockProjection{
		window(t, "a", "p1", "2026-09-02", "2026-09-03", 1),
		window(t, "b", "p1", "2026-09-03", "2026-09-04", 1),
		window(t, "c", "p1", "2026-09-04", "2026-09-05", 1),
	}, 2)

	if len(rows) != 2 || rows[0].BlockID != "a" || rows[1].BlockID != "b" {
		t.Errorf("rows = %+v, want the first two", rows)
	}
}

func TestUpcomingTonnesAddsTheRowsItIsGiven(t *testing.T) {
	rows := upcoming([]agronomy.BlockProjection{
		window(t, "a", "p1", "2026-09-02", "2026-09-03", 1.5),
		window(t, "b", "p1", "2026-09-03", "2026-09-04", 2),
	}, 0)

	if got := dashboard.UpcomingTonnes(rows); got != 3.5 {
		t.Errorf("UpcomingTonnes = %v, want 3.5", got)
	}
}

func TestUpcomingTonnesOfNothingIsZero(t *testing.T) {
	if got := dashboard.UpcomingTonnes(nil); got != 0 {
		t.Errorf("UpcomingTonnes(nil) = %v, want 0", got)
	}
}
