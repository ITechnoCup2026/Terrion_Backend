package planning_test

import (
	"testing"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/planning"
)

func spreadCandidates(t *testing.T, plots int) []planning.PlotCandidate {
	t.Helper()

	windows := [][2]string{
		{"2027-03-01", "2027-03-07"},
		{"2027-03-15", "2027-03-21"},
		{"2027-03-29", "2027-04-04"},
	}

	candidates := make([]planning.PlotCandidate, plots)
	for i := range candidates {
		id := string(rune('a' + i))

		options := make([]planning.Assignment, len(windows))
		for w, window := range windows {
			options[w] = assignmentAt(t, id, "padi", window[0], window[1], 10, 12)
		}

		candidates[i] = planning.PlotCandidate{
			PlotID:     id,
			PlotName:   "Lahan " + id,
			MemberID:   "member-" + id,
			MemberName: "Anggota " + id,
			AreaHa:     1,
			Options:    options,
		}
	}
	return candidates
}

func planFor(t *testing.T, plans []planning.Plan, objective constants.PlanningObjective) planning.Plan {
	t.Helper()

	for _, plan := range plans {
		if plan.Objective == objective {
			return plan
		}
	}
	t.Fatalf("no plan for objective %q", objective)
	return planning.Plan{}
}

func TestSearchReturnsThreePlans(t *testing.T) {
	plans := planning.Search(planning.Input{
		Season: planning.SeasonMT1(2026),
		Plots:  spreadCandidates(t, 4),
	})

	if len(plans) != 3 {
		t.Fatalf("len(plans) = %d, want 3", len(plans))
	}
	wanted := []constants.PlanningObjective{
		constants.ObjectiveSafe, constants.ObjectiveIncome, constants.ObjectiveMarket,
	}
	for i, objective := range wanted {
		if plans[i].Objective != objective {
			t.Errorf("plans[%d].Objective = %q, want %q", i, plans[i].Objective, objective)
		}
	}
}

func TestSearchIsDeterministic(t *testing.T) {
	input := planning.Input{Season: planning.SeasonMT1(2026), Plots: spreadCandidates(t, 5)}

	first := planning.Search(input)
	second := planning.Search(input)

	for p := range first {
		if len(first[p].Assignments) != len(second[p].Assignments) {
			t.Fatalf("plan %d: assignment counts differ", p)
		}
		for a := range first[p].Assignments {
			if first[p].Assignments[a] != second[p].Assignments[a] {
				t.Fatalf("plan %d assignment %d differs:\n  %+v\n  %+v",
					p, a, first[p].Assignments[a], second[p].Assignments[a])
			}
		}
	}
}

func TestSearchAssignsEveryPlotExactlyOnce(t *testing.T) {
	plots := spreadCandidates(t, 6)

	for _, plan := range planning.Search(planning.Input{
		Season: planning.SeasonMT1(2026), Plots: plots,
	}) {
		if len(plan.Assignments) != len(plots) {
			t.Fatalf("plan %q assigned %d plots, want %d",
				plan.Objective, len(plan.Assignments), len(plots))
		}

		seen := map[string]bool{}
		for _, assignment := range plan.Assignments {
			if seen[assignment.PlotID] {
				t.Errorf("plan %q assigned plot %s twice", plan.Objective, assignment.PlotID)
			}
			seen[assignment.PlotID] = true
		}
	}
}

func TestSearchOnlyChoosesOptionsItWasGiven(t *testing.T) {
	plots := spreadCandidates(t, 4)

	offered := map[string]bool{}
	for _, plot := range plots {
		for _, option := range plot.Options {
			offered[option.PlotID+"|"+option.Window.Start.String()] = true
		}
	}

	for _, plan := range planning.Search(planning.Input{
		Season: planning.SeasonMT1(2026), Plots: plots,
	}) {
		for _, assignment := range plan.Assignments {
			if !offered[assignment.PlotID+"|"+assignment.Window.Start.String()] {
				t.Errorf("plan %q invented an option for plot %s",
					plan.Objective, assignment.PlotID)
			}
		}
	}
}

func TestSafePlanNeverHasAWorsePeakThanTheIncomePlan(t *testing.T) {
	plans := planning.Search(planning.Input{
		Season: planning.SeasonMT1(2026),
		Plots:  spreadCandidates(t, 6),
	})

	safe := planFor(t, plans, constants.ObjectiveSafe)
	income := planFor(t, plans, constants.ObjectiveIncome)

	if safe.Metrics.PeakTonnesWorst > income.Metrics.PeakTonnesWorst {
		t.Errorf("safe worst peak = %v, want <= income worst peak %v",
			safe.Metrics.PeakTonnesWorst, income.Metrics.PeakTonnesWorst)
	}
}

func TestSearchStaysUnderTheEvaluationBudget(t *testing.T) {
	plans := planning.Search(planning.Input{
		Season: planning.SeasonMT1(2026),
		Plots:  spreadCandidates(t, 10),
	})

	for _, plan := range plans {
		if plan.Evaluations > constants.PlanningEvaluationBudget {
			t.Errorf("plan %q used %d evaluations, budget is %d",
				plan.Objective, plan.Evaluations, constants.PlanningEvaluationBudget)
		}
	}
}

func TestSearchWithoutPlotsReturnsNoPlans(t *testing.T) {
	plans := planning.Search(planning.Input{Season: planning.SeasonMT1(2026)})

	if len(plans) != 0 {
		t.Errorf("len(plans) = %d, want 0", len(plans))
	}
}
