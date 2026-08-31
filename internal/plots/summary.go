package plots

import (
	"math"
	"sort"

	"terrion-backend/internal/agronomy"
)

type PlotRow struct {
	ID         string
	Name       string
	PublicID   string
	AreaHa     float64
	MemberName *string
}

type PlotSummary struct {
	PlotRow
	BlockCount     int
	NextWindow     *agronomy.HarvestWindow
	ExpectedTonnes *float64
	CommodityIDs   []string
	Progress       *float64
}

func SummarisePlots(
	rows []PlotRow,
	projections []agronomy.BlockProjection,
	windows map[string]agronomy.HarvestWindow,
) []PlotSummary {
	byPlot := map[string][]agronomy.BlockProjection{}
	for _, projection := range projections {
		byPlot[projection.PlotID] = append(byPlot[projection.PlotID], projection)
	}

	summaries := make([]PlotSummary, len(rows))
	for i, row := range rows {
		summaries[i] = summarise(row, byPlot[row.ID], windows)
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return soonerThan(summaries[i].NextWindow, summaries[j].NextWindow)
	})
	return summaries
}

func summarise(
	row PlotRow,
	blocks []agronomy.BlockProjection,
	windows map[string]agronomy.HarvestWindow,
) PlotSummary {
	summary := PlotSummary{
		PlotRow:      row,
		BlockCount:   len(blocks),
		CommodityIDs: []string{},
	}
	if len(blocks) == 0 {
		return summary
	}

	tonnes := 0.0
	seen := map[string]bool{}
	for _, block := range blocks {
		tonnes += block.ExpectedTonnes
		if !seen[block.CommodityID] {
			seen[block.CommodityID] = true
			summary.CommodityIDs = append(summary.CommodityIDs, block.CommodityID)
		}
	}
	summary.ExpectedTonnes = &tonnes

	summary.NextWindow = earliestWindow(blocks, windows)
	if summary.NextWindow != nil && summary.NextWindow.GddRequired > 0 {
		progress := math.Max(0, math.Min(1,
			summary.NextWindow.GddAccumulated/summary.NextWindow.GddRequired))
		summary.Progress = &progress
	}

	return summary
}

func earliestWindow(
	blocks []agronomy.BlockProjection,
	windows map[string]agronomy.HarvestWindow,
) *agronomy.HarvestWindow {
	var earliest *agronomy.HarvestWindow

	for _, block := range blocks {
		window, known := windows[block.BlockID]
		if !known {
			continue
		}
		if earliest == nil || window.Start.Before(earliest.Start) {
			found := window
			earliest = &found
		}
	}
	return earliest
}

func soonerThan(left, right *agronomy.HarvestWindow) bool {
	if left != nil && right != nil {
		return left.Start.Before(right.Start)
	}
	return left != nil
}
