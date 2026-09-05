package planning

import (
	"math"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

type PlotCandidate struct {
	PlotID     string
	PlotName   string
	MemberID   string
	MemberName string
	AreaHa     float64
	Options    []Assignment
}

type Input struct {
	Season     Season
	Plots      []PlotCandidate
	PricePerKg map[string]float64
	Demand     []Demand
	Capacity   map[string]float64
}

type Plan struct {
	Objective   constants.PlanningObjective
	Assignments []Assignment
	Metrics     Metrics
	Flagged     []agronomy.FlaggedWeek
	Evaluations int
	Narrative   string
}

type objectiveSpec struct {
	name      constants.PlanningObjective
	peak      float64
	value     float64
	demand    float64
	worstCase bool
}

var weightedSpecs = []objectiveSpec{
	{constants.ObjectiveSafe, 0.70, 0.20, 0.10, true},
	{constants.ObjectiveIncome, 0.15, 0.75, 0.10, false},
	{constants.ObjectiveMarket, 0.20, 0.20, 0.60, false},
}

var pureSpecs = []objectiveSpec{
	{constants.ObjectiveSafe, 1, 0, 0, true},
	{constants.ObjectiveIncome, 0, 1, 0, false},
	{constants.ObjectiveMarket, 0, 0, 1, false},
}

type bounds struct {
	peakLow    float64
	peakHigh   float64
	valueLow   float64
	valueHigh  float64
	demandLow  float64
	demandHigh float64
}

func rawBounds() bounds {
	return bounds{
		peakLow: math.Inf(1), valueLow: math.Inf(1), demandLow: math.Inf(1),
		peakHigh: math.Inf(-1), valueHigh: math.Inf(-1), demandHigh: math.Inf(-1),
	}
}

func identityBounds() bounds {
	return bounds{peakHigh: 1, valueHigh: 1, demandHigh: 1}
}

func widen(limits bounds, metrics Metrics) bounds {
	value := 0.0
	if metrics.GrossValue != nil {
		value = *metrics.GrossValue
	}

	limits.peakLow = math.Min(limits.peakLow, metrics.PeakTonnesExpected)
	limits.peakHigh = math.Max(limits.peakHigh, metrics.PeakTonnesWorst)
	limits.valueLow = math.Min(limits.valueLow, value)
	limits.valueHigh = math.Max(limits.valueHigh, value)
	limits.demandLow = math.Min(limits.demandLow, metrics.DemandCoveredKg)
	limits.demandHigh = math.Max(limits.demandHigh, metrics.DemandCoveredKg)
	return limits
}

func span(value, low, high float64) float64 {
	if high-low < 1e-9 {
		return 0
	}
	return (value - low) / (high - low)
}

func scalarise(metrics Metrics, spec objectiveSpec, limits bounds) float64 {
	peak := metrics.PeakTonnesExpected
	if spec.worstCase {
		peak = metrics.PeakTonnesWorst
	}

	value := 0.0
	if metrics.GrossValue != nil {
		value = span(*metrics.GrossValue, limits.valueLow, limits.valueHigh)
	}

	return spec.peak*(1-span(peak, limits.peakLow, limits.peakHigh)) +
		spec.value*value +
		spec.demand*span(metrics.DemandCoveredKg, limits.demandLow, limits.demandHigh)
}

func greedy(input Input, spec objectiveSpec, limits bounds) ([]Assignment, int) {
	chosen := make([]Assignment, 0, len(input.Plots))
	evaluations := 0

	for _, plot := range input.Plots {
		bestIndex := -1
		bestScore := math.Inf(-1)

		for i, option := range plot.Options {
			trial := make([]Assignment, len(chosen), len(chosen)+1)
			copy(trial, chosen)
			trial = append(trial, option)

			score := scalarise(Measure(trial, input.PricePerKg, input.Demand), spec, limits)
			evaluations++

			if score > bestScore {
				bestScore = score
				bestIndex = i
			}
		}

		if bestIndex >= 0 {
			chosen = append(chosen, plot.Options[bestIndex])
		}
	}
	return chosen, evaluations
}

func improve(
	input Input, spec objectiveSpec, limits bounds, seed []Assignment,
) ([]Assignment, int) {
	current := make([]Assignment, len(seed))
	copy(current, seed)

	positionOfPlot := make(map[string]int, len(current))
	for i, assignment := range current {
		positionOfPlot[assignment.PlotID] = i
	}

	evaluations := 0
	currentScore := scalarise(Measure(current, input.PricePerKg, input.Demand), spec, limits)

	for range constants.PlanningLocalSearchPasses {
		bestScore := currentScore
		bestPosition := -1
		bestOption := Assignment{}

		for _, plot := range input.Plots {
			position, placed := positionOfPlot[plot.PlotID]
			if !placed {
				continue
			}

			for _, option := range plot.Options {
				if option.Window.Start.Equal(current[position].Window.Start) &&
					option.VarietyID == current[position].VarietyID {
					continue
				}

				trial := make([]Assignment, len(current))
				copy(trial, current)
				trial[position] = option

				score := scalarise(Measure(trial, input.PricePerKg, input.Demand), spec, limits)
				evaluations++

				if score > bestScore {
					bestScore = score
					bestPosition = position
					bestOption = option
				}
			}
		}

		if bestPosition < 0 {
			break
		}
		current[bestPosition] = bestOption
		currentScore = bestScore
	}

	return current, evaluations
}

func Search(input Input) []Plan {
	if len(input.Plots) == 0 {
		return []Plan{}
	}

	limits := rawBounds()
	evaluations := 0

	for _, spec := range pureSpecs {
		seed, used := greedy(input, spec, identityBounds())
		evaluations += used
		limits = widen(limits, Measure(seed, input.PricePerKg, input.Demand))
	}

	plans := make([]Plan, 0, len(weightedSpecs))
	for _, spec := range weightedSpecs {
		seed, used := greedy(input, spec, limits)
		evaluations += used

		improved, more := improve(input, spec, limits, seed)
		evaluations += more

		plans = append(plans, Plan{
			Objective:   spec.name,
			Assignments: improved,
			Metrics:     Measure(improved, input.PricePerKg, input.Demand),
			Flagged: agronomy.DetectCollisions(
				Projections(improved), input.Capacity).Flagged,
		})
	}

	for i := range plans {
		plans[i].Evaluations = evaluations
	}
	return plans
}
