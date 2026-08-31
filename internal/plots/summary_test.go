package plots_test

import (
	"math"
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/plots"
)

func date(t *testing.T, iso string) time.Time {
	t.Helper()
	parsed, err := agronomy.UTCDate(iso)
	if err != nil {
		t.Fatalf("UTCDate(%q): %v", iso, err)
	}
	return parsed
}

func plotRow(id, name string, areaHa float64, memberName *string) plots.PlotRow {
	return plots.PlotRow{ID: id, Name: name, PublicID: "pub-" + id, AreaHa: areaHa, MemberName: memberName}
}

func blockProjection(
	t *testing.T, blockID, plotID, start, end string, tonnes float64,
) agronomy.BlockProjection {
	t.Helper()
	return agronomy.BlockProjection{
		BlockID:     blockID,
		PlotID:      plotID,
		CommodityID: "c1",
		Window: agronomy.DateRange{
			Start: date(t, start), End: date(t, end),
		},
		ExpectedTonnes: tonnes,
	}
}

func harvestWindow(t *testing.T, start, end string) agronomy.HarvestWindow {
	t.Helper()
	return agronomy.HarvestWindow{
		Start: date(t, start), End: date(t, end),
		Confidence:     constants.HarvestWindowConfidence,
		GddAccumulated: 100, GddRequired: 200,
		Stage: constants.StageVegetative,
		Basis: constants.BasisObserved, Plausibility: constants.PlausibilityOk,
	}
}

func TestSummarisePlotsTotalsAreaAndTonnage(t *testing.T) {
	asep := "Pak Asep"

	summaries := plots.SummarisePlots(
		[]plots.PlotRow{plotRow("p1", "Sawah Utara", 1.4, &asep)},
		[]agronomy.BlockProjection{
			blockProjection(t, "b1", "p1", "2026-09-07", "2026-09-13", 3),
			blockProjection(t, "b2", "p1", "2026-09-14", "2026-09-20", 2),
		},
		map[string]agronomy.HarvestWindow{"b1": harvestWindow(t, "2026-09-07", "2026-09-13")},
	)

	summary := summaries[0]
	if summary.Name != "Sawah Utara" || summary.MemberName == nil || *summary.MemberName != asep {
		t.Errorf("summary = %+v, want Sawah Utara / Pak Asep", summary)
	}
	if summary.BlockCount != 2 {
		t.Errorf("BlockCount = %d, want 2", summary.BlockCount)
	}
	if summary.ExpectedTonnes == nil || math.Abs(*summary.ExpectedTonnes-5) > 5e-6 {
		t.Errorf("ExpectedTonnes = %v, want 5", summary.ExpectedTonnes)
	}
}

func TestSummarisePlotsTakesTheEarliestWindowOfAPlot(t *testing.T) {
	summaries := plots.SummarisePlots(
		[]plots.PlotRow{plotRow("p1", "Sawah", 1, nil)},
		[]agronomy.BlockProjection{
			blockProjection(t, "late", "p1", "2026-10-05", "2026-10-11", 1),
			blockProjection(t, "early", "p1", "2026-09-07", "2026-09-13", 1),
		},
		map[string]agronomy.HarvestWindow{
			"late":  harvestWindow(t, "2026-10-05", "2026-10-11"),
			"early": harvestWindow(t, "2026-09-07", "2026-09-13"),
		},
	)

	if summaries[0].NextWindow == nil ||
		!summaries[0].NextWindow.Start.Equal(date(t, "2026-09-07")) {
		t.Errorf("NextWindow = %+v, want the 2026-09-07 window", summaries[0].NextWindow)
	}
}

func TestSummarisePlotsSortsBySoonestHarvest(t *testing.T) {
	summaries := plots.SummarisePlots(
		[]plots.PlotRow{plotRow("p1", "Nanti", 1, nil), plotRow("p2", "Duluan", 1, nil)},
		[]agronomy.BlockProjection{
			blockProjection(t, "b1", "p1", "2026-10-05", "2026-10-11", 1),
			blockProjection(t, "b2", "p2", "2026-09-07", "2026-09-13", 1),
		},
		map[string]agronomy.HarvestWindow{
			"b1": harvestWindow(t, "2026-10-05", "2026-10-11"),
			"b2": harvestWindow(t, "2026-09-07", "2026-09-13"),
		},
	)

	if summaries[0].Name != "Duluan" || summaries[1].Name != "Nanti" {
		t.Errorf("order = %q, %q, want Duluan then Nanti", summaries[0].Name, summaries[1].Name)
	}
}

func TestSummarisePlotsKeepsUnprojectedPlotsAtTheEnd(t *testing.T) {
	summaries := plots.SummarisePlots(
		[]plots.PlotRow{plotRow("p1", "Belum ditanam", 1, nil), plotRow("p2", "Sudah", 1, nil)},
		[]agronomy.BlockProjection{blockProjection(t, "b1", "p2", "2026-09-07", "2026-09-13", 4)},
		map[string]agronomy.HarvestWindow{"b1": harvestWindow(t, "2026-09-07", "2026-09-13")},
	)

	if summaries[0].Name != "Sudah" || summaries[1].Name != "Belum ditanam" {
		t.Fatalf("order = %q, %q, want Sudah then Belum ditanam",
			summaries[0].Name, summaries[1].Name)
	}
	if summaries[1].NextWindow != nil {
		t.Error("NextWindow of an unplanted plot is set, want nil")
	}
	if summaries[1].BlockCount != 0 {
		t.Errorf("BlockCount = %d, want 0", summaries[1].BlockCount)
	}
	if summaries[1].ExpectedTonnes != nil {
		t.Errorf("ExpectedTonnes = %v, want nil rather than a misleading zero",
			*summaries[1].ExpectedTonnes)
	}
}

func TestSummarisePlotsReportsNoWindowWhenTheProjectionHasNone(t *testing.T) {
	summaries := plots.SummarisePlots(
		[]plots.PlotRow{plotRow("p1", "Sawah", 1, nil)},
		[]agronomy.BlockProjection{blockProjection(t, "b1", "p1", "2026-09-07", "2026-09-13", 4)},
		nil,
	)

	if summaries[0].NextWindow != nil {
		t.Error("NextWindow is set, want nil when the model produced no window")
	}
	if summaries[0].ExpectedTonnes == nil || math.Abs(*summaries[0].ExpectedTonnes-4) > 5e-6 {
		t.Errorf("ExpectedTonnes = %v, want 4", summaries[0].ExpectedTonnes)
	}
	if summaries[0].Progress != nil {
		t.Error("Progress is set, want nil without a window")
	}
}

func TestSummarisePlotsClampsProgress(t *testing.T) {
	window := harvestWindow(t, "2026-09-07", "2026-09-13")
	window.GddAccumulated = 280
	window.GddRequired = 200

	summaries := plots.SummarisePlots(
		[]plots.PlotRow{plotRow("p1", "Sawah", 1, nil)},
		[]agronomy.BlockProjection{blockProjection(t, "b1", "p1", "2026-09-07", "2026-09-13", 4)},
		map[string]agronomy.HarvestWindow{"b1": window},
	)

	if summaries[0].Progress == nil || *summaries[0].Progress != 1 {
		t.Errorf("Progress = %v, want 1: a meter reading 140%% says the opposite of ready",
			summaries[0].Progress)
	}
}
