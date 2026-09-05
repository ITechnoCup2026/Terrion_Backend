package planning

import (
	"fmt"
	"sort"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

const (
	DefaultMaxCandidates = 2000
	KilogramsPerTonne    = 1000.0
)

type Plot struct {
	ID     string
	AreaHa float64
}

type Variety struct {
	ID          string
	CommodityID string
	Agronomy    agronomy.Variety
}

type Season struct {
	Label string
	Start time.Time
	End   time.Time
}

type Candidate struct {
	ID           string
	PlotID       string
	AreaHa       float64
	CommodityID  string
	VarietyID    string
	PlantingDate time.Time
	HarvestStart time.Time
	HarvestEnd   time.Time
	TonnesLow    float64
	TonnesMid    float64
	TonnesHigh   float64
	Plausibility constants.Plausibility
	PricePerKg   *float64
}

type CandidateInput struct {
	Season        Season
	Plots         []Plot
	Varieties     []Variety
	Observed      []agronomy.TempDay
	Forecast      []agronomy.TempDay
	Climatology   []agronomy.ClimateNormal
	Calibrations  map[string]agronomy.Calibration
	YieldModel    agronomy.YieldModel
	PricePerKg    map[string]*float64
	MaxCandidates int
}

func plantingWeeks(season Season, variety agronomy.Variety) []time.Time {
	weeks := make([]time.Time, 0, 32)
	cursor := agronomy.ISOWeekStart(season.Start)
	if cursor.Before(agronomy.StartOfDay(season.Start)) {
		cursor = agronomy.AddDays(cursor, 7)
	}

	for ; !cursor.After(season.End); cursor = agronomy.AddDays(cursor, 7) {
		if agronomy.AddDays(cursor, variety.DaysToHarvestMin).After(season.End) {
			break
		}
		weeks = append(weeks, cursor)
	}
	return weeks
}

func BuildCandidates(input CandidateInput) []Candidate {
	limit := input.MaxCandidates
	if limit <= 0 {
		limit = DefaultMaxCandidates
	}

	plots := append([]Plot(nil), input.Plots...)
	sort.Slice(plots, func(i, j int) bool { return plots[i].ID < plots[j].ID })

	varieties := append([]Variety(nil), input.Varieties...)
	sort.Slice(varieties, func(i, j int) bool { return varieties[i].ID < varieties[j].ID })

	weather := append(append([]agronomy.TempDay(nil), input.Observed...), input.Forecast...)

	candidates := make([]Candidate, 0, 128)
	for _, plot := range plots {
		if plot.AreaHa <= 0 {
			continue
		}

		for _, variety := range varieties {
			var calibration *agronomy.Calibration
			if fitted, ok := input.Calibrations[variety.ID]; ok {
				calibration = &fitted
			}

			for _, planting := range plantingWeeks(input.Season, variety.Agronomy) {
				candidate, ok := buildOne(input, plot, variety, calibration, weather, planting)
				if !ok {
					continue
				}
				candidates = append(candidates, candidate)
				if len(candidates) >= limit {
					return numbered(candidates)
				}
			}
		}
	}

	return numbered(candidates)
}

func buildOne(
	input CandidateInput,
	plot Plot,
	variety Variety,
	calibration *agronomy.Calibration,
	weather []agronomy.TempDay,
	planting time.Time,
) (Candidate, bool) {
	window, err := agronomy.PredictHarvest(agronomy.HarvestInput{
		PlantingDate: planting,
		Observed:     input.Observed,
		Forecast:     input.Forecast,
		Climatology:  input.Climatology,
		Variety:      variety.Agronomy,
		Calibration:  calibration,
	})
	if err != nil || agronomy.IsImplausible(window) {
		return Candidate{}, false
	}
	if window.End.After(input.Season.End) {
		return Candidate{}, false
	}

	features := agronomy.DeriveYieldFeatures(agronomy.YieldFeaturesInput{
		PlantingDate: planting,
		ThroughDate:  window.End,
		AreaHa:       plot.AreaHa,
		Variety:      variety.Agronomy,
		Weather:      SeriesForWindow(weather, input.Climatology, planting, window.End),
	})

	perHa := agronomy.PredictYieldRange(input.YieldModel, features, variety.Agronomy)

	return Candidate{
		PlotID:       plot.ID,
		AreaHa:       plot.AreaHa,
		CommodityID:  variety.CommodityID,
		VarietyID:    variety.ID,
		PlantingDate: planting,
		HarvestStart: window.Start,
		HarvestEnd:   window.End,
		TonnesLow:    perHa.Low * plot.AreaHa,
		TonnesMid:    perHa.Mid * plot.AreaHa,
		TonnesHigh:   perHa.High * plot.AreaHa,
		Plausibility: window.Plausibility,
		PricePerKg:   input.PricePerKg[variety.CommodityID],
	}, true
}

func numbered(candidates []Candidate) []Candidate {
	for i := range candidates {
		candidates[i].ID = fmt.Sprintf("c%03d", i+1)
	}
	return candidates
}

func MergeCandidates(groups ...[]Candidate) []Candidate {
	merged := []Candidate{}
	for _, group := range groups {
		merged = append(merged, group...)
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].PlotID != merged[j].PlotID {
			return merged[i].PlotID < merged[j].PlotID
		}
		if merged[i].VarietyID != merged[j].VarietyID {
			return merged[i].VarietyID < merged[j].VarietyID
		}
		return merged[i].PlantingDate.Before(merged[j].PlantingDate)
	})

	return numbered(merged)
}
