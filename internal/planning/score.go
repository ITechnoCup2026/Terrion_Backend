package planning

import (
	"math"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

type Assignment struct {
	PlotID       string
	PlotName     string
	MemberID     string
	MemberName   string
	AreaHa       float64
	CommodityID  string
	VarietyID    string
	VarietyName  string
	PlantingDate time.Time
	Window       agronomy.DateRange
	Plausibility constants.Plausibility
	TonnesLow    float64
	TonnesMid    float64
	TonnesHigh   float64
}

type Metrics struct {
	PeakTonnesExpected float64
	PeakTonnesWorst    float64
	GrossValue         *float64
	DemandCoveredKg    float64
	TotalTonnesMid     float64
}

func Projections(assignments []Assignment) []agronomy.BlockProjection {
	projections := make([]agronomy.BlockProjection, len(assignments))
	for i, assignment := range assignments {
		projections[i] = agronomy.BlockProjection{
			BlockID:        assignment.PlotID,
			PlotID:         assignment.PlotID,
			CommodityID:    assignment.CommodityID,
			Window:         assignment.Window,
			ExpectedTonnes: assignment.TonnesMid,
		}
	}
	return projections
}

func collapsedProjections(assignments []Assignment) []agronomy.BlockProjection {
	projections := make([]agronomy.BlockProjection, len(assignments))
	for i, assignment := range assignments {
		middle := agronomy.AddDays(assignment.Window.Start,
			agronomy.DaysBetween(assignment.Window.Start, assignment.Window.End)/2)

		projections[i] = agronomy.BlockProjection{
			BlockID:        assignment.PlotID,
			PlotID:         assignment.PlotID,
			CommodityID:    assignment.CommodityID,
			Window:         agronomy.DateRange{Start: middle, End: middle},
			ExpectedTonnes: assignment.TonnesHigh,
		}
	}
	return projections
}

func peakOf(projections []agronomy.BlockProjection) float64 {
	peak := 0.0
	for _, week := range agronomy.BucketByWeek(projections) {
		if week.Tonnes > peak {
			peak = week.Tonnes
		}
	}
	return peak
}

func Measure(
	assignments []Assignment, pricePerKg map[string]float64, demand []Demand,
) Metrics {
	total := 0.0
	for _, assignment := range assignments {
		total += assignment.TonnesMid
	}

	return Metrics{
		PeakTonnesExpected: peakOf(Projections(assignments)),
		PeakTonnesWorst:    peakOf(collapsedProjections(assignments)),
		GrossValue:         grossValueOf(assignments, pricePerKg),
		DemandCoveredKg:    demandCoveredOf(assignments, demand),
		TotalTonnesMid:     total,
	}
}

func grossValueOf(assignments []Assignment, pricePerKg map[string]float64) *float64 {
	if len(assignments) == 0 {
		return nil
	}

	total := 0.0
	for _, assignment := range assignments {
		price, published := pricePerKg[assignment.CommodityID]
		if !published {
			return nil
		}
		total += assignment.TonnesMid * constants.KgPerTonne * price
	}
	return &total
}

func demandCoveredOf(assignments []Assignment, demand []Demand) float64 {
	if len(demand) == 0 {
		return 0
	}

	suppliedKg := map[string]float64{}
	for _, week := range agronomy.BucketByWeek(Projections(assignments)) {
		suppliedKg[week.CommodityID+"|"+week.ISOWeek] = week.Tonnes * constants.KgPerTonne
	}

	covered := 0.0
	for _, wanted := range demand {
		covered += math.Min(wanted.Kg, suppliedKg[wanted.CommodityID+"|"+wanted.ISOWeek])
	}
	return covered
}
