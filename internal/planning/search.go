package planning

import (
	"sort"
	"time"

	"terrion-backend/internal/agronomy"
)

const (
	scoreTolerance = 1e-12
	improvePasses  = 3
)

type Objective string

const (
	ObjectiveSafe   Objective = "aman"
	ObjectiveIncome Objective = "pendapatan"
	ObjectiveMarket Objective = "pasar"
)

var objectiveWeights = map[Objective][3]float64{
	ObjectiveSafe:   {0.70, 0.20, 0.10},
	ObjectiveIncome: {0.15, 0.75, 0.10},
	ObjectiveMarket: {0.20, 0.20, 0.60},
}

var objectiveUsesWorstCase = map[Objective]bool{
	ObjectiveSafe:   true,
	ObjectiveIncome: false,
	ObjectiveMarket: false,
}

type DemandRow struct {
	CommodityID string
	Week        time.Time
	Kg          int
}

type Measures struct {
	Peak        float64
	Income      float64
	CoverageKg  int
	TotalTonnes float64
	GrossValue  *float64
}

type Plan struct {
	Objective     Objective
	CandidateIDs  []string
	Metrics       Measures
	ExpectedPeak  float64
	WorstCasePeak float64
}

type demandSlot struct {
	commodityID string
	week        int
}

type Problem struct {
	weeks      []string
	weekIndex  map[string]int
	candidates []Candidate
	byID       map[string]Candidate
	byPlot     map[string][]Candidate
	plotIDs    []string
	span       map[string][2]int
	demandKg   map[demandSlot]int
	capacity   *float64
}

func WeekKey(day time.Time) string {
	return agronomy.ToISODate(agronomy.ISOWeekStart(day))
}

func NewProblem(candidates []Candidate, demand []DemandRow, capacity *float64) *Problem {
	ordered := append([]Candidate(nil), candidates...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	keys := map[string]bool{}
	for _, candidate := range ordered {
		for cursor := agronomy.ISOWeekStart(candidate.HarvestStart); !cursor.After(candidate.HarvestEnd); cursor = agronomy.AddDays(cursor, 7) {
			keys[agronomy.ToISODate(cursor)] = true
		}
	}
	for _, row := range demand {
		keys[WeekKey(row.Week)] = true
	}

	weeks := make([]string, 0, len(keys))
	for key := range keys {
		weeks = append(weeks, key)
	}
	sort.Strings(weeks)

	weekIndex := make(map[string]int, len(weeks))
	for i, key := range weeks {
		weekIndex[key] = i
	}

	span := make(map[string][2]int, len(ordered))
	byID := make(map[string]Candidate, len(ordered))
	byPlot := map[string][]Candidate{}
	for _, candidate := range ordered {
		span[candidate.ID] = [2]int{
			weekIndex[WeekKey(candidate.HarvestStart)],
			weekIndex[WeekKey(candidate.HarvestEnd)],
		}
		byID[candidate.ID] = candidate
		byPlot[candidate.PlotID] = append(byPlot[candidate.PlotID], candidate)
	}

	plotIDs := make([]string, 0, len(byPlot))
	for id := range byPlot {
		plotIDs = append(plotIDs, id)
	}
	sort.Strings(plotIDs)

	demandKg := map[demandSlot]int{}
	for _, row := range demand {
		slot := demandSlot{commodityID: row.CommodityID, week: weekIndex[WeekKey(row.Week)]}
		demandKg[slot] += row.Kg
	}

	return &Problem{
		weeks: weeks, weekIndex: weekIndex, candidates: ordered, byID: byID,
		byPlot: byPlot, plotIDs: plotIDs, span: span, demandKg: demandKg, capacity: capacity,
	}
}

func (p *Problem) Candidates() []Candidate { return p.candidates }

func (p *Problem) Select(ids []string) []Candidate {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)

	chosen := make([]Candidate, 0, len(sorted))
	for _, id := range sorted {
		if candidate, ok := p.byID[id]; ok {
			chosen = append(chosen, candidate)
		}
	}
	return chosen
}

func (p *Problem) weekShare(candidate Candidate, worst bool) map[int]float64 {
	bounds := p.span[candidate.ID]
	weeks := bounds[1] - bounds[0] + 1

	if worst {
		return map[int]float64{bounds[0] + weeks/2: candidate.TonnesHigh}
	}

	share := make(map[int]float64, weeks)
	for week := bounds[0]; week <= bounds[1]; week++ {
		share[week] = candidate.TonnesMid / float64(weeks)
	}
	return share
}

func (p *Problem) weeklyTotals(chosen []Candidate, worst bool) []float64 {
	totals := make([]float64, len(p.weeks))
	for _, candidate := range chosen {
		for week, tonnes := range p.weekShare(candidate, worst) {
			totals[week] += tonnes
		}
	}
	return totals
}

func (p *Problem) Measure(chosen []Candidate, worst bool) Measures {
	if len(chosen) == 0 {
		zero := 0.0
		return Measures{GrossValue: &zero}
	}

	peak := 0.0
	for _, total := range p.weeklyTotals(chosen, worst) {
		if total > peak {
			peak = total
		}
	}

	supply := map[demandSlot]float64{}
	income := 0.0
	totalTonnes := 0.0
	priced := 0

	for _, candidate := range chosen {
		totalTonnes += candidate.TonnesMid
		if candidate.PricePerKg != nil {
			income += candidate.TonnesMid * KilogramsPerTonne * *candidate.PricePerKg
			priced++
		}
		for week, tonnes := range p.weekShare(candidate, false) {
			slot := demandSlot{commodityID: candidate.CommodityID, week: week}
			supply[slot] += tonnes * KilogramsPerTonne
		}
	}

	covered := 0.0
	for slot, wanted := range p.demandKg {
		if available := supply[slot]; available < float64(wanted) {
			covered += available
		} else {
			covered += float64(wanted)
		}
	}

	measures := Measures{
		Peak: peak, Income: income, CoverageKg: int(covered), TotalTonnes: totalTonnes,
	}
	if priced == len(chosen) {
		value := income
		measures.GrossValue = &value
	}
	return measures
}

type bounds struct {
	peakExpected [2]float64
	peakWorst    [2]float64
	income       [2]float64
	coverage     [2]float64
}

func normalise(value float64, span [2]float64) float64 {
	if span[1]-span[0] < 1e-9 {
		return 0
	}
	scaled := (value - span[0]) / (span[1] - span[0])
	return min(1, max(0, scaled))
}

func (b bounds) peakSpan(objective Objective) [2]float64 {
	if objectiveUsesWorstCase[objective] {
		return b.peakWorst
	}
	return b.peakExpected
}

func scalarise(measures Measures, objective Objective, b bounds) float64 {
	weights := objectiveWeights[objective]
	return weights[0]*(1-normalise(measures.Peak, b.peakSpan(objective))) +
		weights[1]*normalise(measures.Income, b.income) +
		weights[2]*normalise(float64(measures.CoverageKg), b.coverage)
}

func criterionValue(measures Measures, criterion int) float64 {
	switch criterion {
	case 0:
		return -measures.Peak
	case 1:
		return measures.Income
	default:
		return float64(measures.CoverageKg)
	}
}

func (p *Problem) probe(criterion int, worst bool) Measures {
	chosen := make([]Candidate, 0, len(p.plotIDs))
	for _, plotID := range p.plotIDs {
		best, bestValue, found := Candidate{}, 0.0, false
		for _, candidate := range p.byPlot[plotID] {
			value := criterionValue(p.Measure(append(chosen, candidate), worst), criterion)
			if !found || value > bestValue+scoreTolerance {
				best, bestValue, found = candidate, value, true
			}
		}
		if found {
			chosen = append(chosen, best)
		}
	}
	return p.Measure(chosen, worst)
}

func (p *Problem) deriveBounds() bounds {
	expected := [3]Measures{p.probe(0, false), p.probe(1, false), p.probe(2, false)}
	worst := [3]Measures{p.probe(0, true), p.probe(1, true), p.probe(2, true)}

	spanOf := func(values [3]float64) [2]float64 {
		low, high := values[0], values[0]
		for _, value := range values[1:] {
			low, high = min(low, value), max(high, value)
		}
		return [2]float64{low, high}
	}

	return bounds{
		peakExpected: spanOf([3]float64{expected[0].Peak, expected[1].Peak, expected[2].Peak}),
		peakWorst:    spanOf([3]float64{worst[0].Peak, worst[1].Peak, worst[2].Peak}),
		income:       spanOf([3]float64{expected[0].Income, expected[1].Income, expected[2].Income}),
		coverage: spanOf([3]float64{
			float64(expected[0].CoverageKg),
			float64(expected[1].CoverageKg),
			float64(expected[2].CoverageKg),
		}),
	}
}

func (p *Problem) score(chosen []Candidate, objective Objective, b bounds) float64 {
	return scalarise(p.Measure(chosen, objectiveUsesWorstCase[objective]), objective, b)
}

func (p *Problem) solveOne(objective Objective, b bounds) ([]Candidate, int) {
	evaluations := 0
	chosen := make([]Candidate, 0, len(p.plotIDs))

	for _, plotID := range p.plotIDs {
		best, bestScore, found := Candidate{}, 0.0, false
		for _, candidate := range p.byPlot[plotID] {
			value := p.score(append(chosen, candidate), objective, b)
			evaluations++
			if !found || value > bestScore+scoreTolerance {
				best, bestScore, found = candidate, value, true
			}
		}
		if found {
			chosen = append(chosen, best)
		}
	}

	current := p.score(chosen, objective, b)
	for range improvePasses {
		moved := false
		for position, existing := range chosen {
			for _, alternative := range p.byPlot[existing.PlotID] {
				if alternative.ID == existing.ID {
					continue
				}
				trial := append([]Candidate(nil), chosen...)
				trial[position] = alternative
				value := p.score(trial, objective, b)
				evaluations++
				if value > current+scoreTolerance {
					chosen, current, moved = trial, value, true
				}
			}
		}
		if !moved {
			break
		}
	}

	return chosen, evaluations
}

func Search(problem *Problem, objectives []Objective) ([]Plan, int) {
	b := problem.deriveBounds()
	plans := make([]Plan, 0, len(objectives))
	evaluations := 0

	for _, objective := range objectives {
		chosen, count := problem.solveOne(objective, b)
		evaluations += count

		ids := make([]string, 0, len(chosen))
		for _, candidate := range chosen {
			ids = append(ids, candidate.ID)
		}
		sort.Strings(ids)

		expected := problem.Measure(chosen, false)
		plans = append(plans, Plan{
			Objective:     objective,
			CandidateIDs:  ids,
			Metrics:       expected,
			ExpectedPeak:  expected.Peak,
			WorstCasePeak: problem.Measure(chosen, true).Peak,
		})
	}

	return plans, evaluations
}
