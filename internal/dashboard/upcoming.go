package dashboard

import (
	"sort"
	"time"

	"terrion-backend/internal/agronomy"
)

type PlotRef struct {
	Name       string
	MemberName *string
}

type UpcomingHarvest struct {
	BlockID       string
	PlotID        string
	PlotName      string
	MemberName    *string
	CommodityName string
	Tonnes        float64
	Start         time.Time
	End           time.Time
}

func UpcomingHarvests(
	projections []agronomy.BlockProjection,
	from, to time.Time,
	plots map[string]PlotRef,
	commodities map[string]string,
	limit int,
) []UpcomingHarvest {
	rows := []UpcomingHarvest{}

	for _, projection := range projections {
		if projection.Window.Start.After(to) || projection.Window.End.Before(from) {
			continue
		}
		plot, visible := plots[projection.PlotID]
		if !visible {
			continue
		}

		name, known := commodities[projection.CommodityID]
		if !known {
			name = "Komoditas"
		}

		rows = append(rows, UpcomingHarvest{
			BlockID:       projection.BlockID,
			PlotID:        projection.PlotID,
			PlotName:      plot.Name,
			MemberName:    plot.MemberName,
			CommodityName: name,
			Tonnes:        projection.ExpectedTonnes,
			Start:         projection.Window.Start,
			End:           projection.Window.End,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Start.Before(rows[j].Start)
	})

	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func UpcomingTonnes(rows []UpcomingHarvest) float64 {
	total := 0.0
	for _, row := range rows {
		total += row.Tonnes
	}
	return total
}
