package planning_test

import (
	"reflect"
	"testing"

	"terrion-backend/internal/planning"
)

func sampleProblem(t *testing.T) (*planning.Problem, []planning.Candidate) {
	t.Helper()

	candidates := planning.BuildCandidates(sampleInput(t))
	if len(candidates) < 8 {
		t.Fatalf("perlu kandidat yang cukup untuk diuji, dapat %d", len(candidates))
	}

	demand := []planning.DemandRow{
		{CommodityID: "padi", Week: candidates[0].HarvestStart, Kg: 3000},
		{CommodityID: "padi", Week: candidates[len(candidates)-1].HarvestStart, Kg: 2000},
	}
	capacity := 12.5

	return planning.NewProblem(candidates, demand, &capacity), candidates
}

func allObjectives() []planning.Objective {
	return []planning.Objective{
		planning.ObjectiveSafe, planning.ObjectiveIncome, planning.ObjectiveMarket,
	}
}

func TestSearchReturnsOnePlanPerObjectiveInOrder(t *testing.T) {
	problem, _ := sampleProblem(t)

	plans, _ := planning.Search(problem, allObjectives())

	if len(plans) != 3 {
		t.Fatalf("harus tiga rencana, dapat %d", len(plans))
	}
	for i, objective := range allObjectives() {
		if plans[i].Objective != objective {
			t.Fatalf("urutan objektif berubah pada posisi %d: %s", i, plans[i].Objective)
		}
	}
}

func TestSearchPlantsEachPlotAtMostOnce(t *testing.T) {
	problem, _ := sampleProblem(t)

	plans, _ := planning.Search(problem, allObjectives())

	for _, plan := range plans {
		seen := map[string]bool{}
		for _, candidate := range problem.Select(plan.CandidateIDs) {
			if seen[candidate.PlotID] {
				t.Fatalf("%s menanami %s dua kali", plan.Objective, candidate.PlotID)
			}
			seen[candidate.PlotID] = true
		}
	}
}

func TestSearchIsDeterministic(t *testing.T) {
	problem, _ := sampleProblem(t)

	first, firstCount := planning.Search(problem, allObjectives())
	second, secondCount := planning.Search(problem, allObjectives())

	if !reflect.DeepEqual(first, second) || firstCount != secondCount {
		t.Fatal("dua pencarian atas soal yang sama menghasilkan rencana yang berbeda")
	}
}

func TestSearchIgnoresTheOrderCandidatesArriveIn(t *testing.T) {
	problem, candidates := sampleProblem(t)
	forward, _ := planning.Search(problem, allObjectives())

	reversed := make([]planning.Candidate, 0, len(candidates))
	for i := len(candidates) - 1; i >= 0; i-- {
		reversed = append(reversed, candidates[i])
	}
	capacity := 12.5
	demand := []planning.DemandRow{
		{CommodityID: "padi", Week: candidates[0].HarvestStart, Kg: 3000},
		{CommodityID: "padi", Week: candidates[len(candidates)-1].HarvestStart, Kg: 2000},
	}
	backward, _ := planning.Search(planning.NewProblem(reversed, demand, &capacity), allObjectives())

	if !reflect.DeepEqual(forward, backward) {
		t.Fatal("urutan kandidat mengubah rencana; determinisme bocor lewat urutan kueri")
	}
}

func TestDominanceInvariantsHold(t *testing.T) {
	problem, _ := sampleProblem(t)

	plans, _ := planning.Search(problem, allObjectives())
	byObjective := map[planning.Objective]planning.Plan{}
	for _, plan := range plans {
		byObjective[plan.Objective] = plan
	}

	safe := byObjective[planning.ObjectiveSafe]
	income := byObjective[planning.ObjectiveIncome]
	market := byObjective[planning.ObjectiveMarket]

	if safe.WorstCasePeak > income.WorstCasePeak || safe.WorstCasePeak > market.WorstCasePeak {
		t.Fatalf("rencana aman tidak punya puncak kasus terburuk terendah: aman %v pendapatan %v pasar %v",
			safe.WorstCasePeak, income.WorstCasePeak, market.WorstCasePeak)
	}
	if income.Metrics.Income < safe.Metrics.Income || income.Metrics.Income < market.Metrics.Income {
		t.Fatalf("rencana pendapatan tidak punya nilai tertinggi: aman %v pendapatan %v pasar %v",
			safe.Metrics.Income, income.Metrics.Income, market.Metrics.Income)
	}
	if market.Metrics.CoverageKg < safe.Metrics.CoverageKg || market.Metrics.CoverageKg < income.Metrics.CoverageKg {
		t.Fatalf("rencana pasar tidak punya cakupan tertinggi: aman %d pendapatan %d pasar %d",
			safe.Metrics.CoverageKg, income.Metrics.CoverageKg, market.Metrics.CoverageKg)
	}
}

func TestWorstCasePeakNeverUndercutsTheExpectedPeak(t *testing.T) {
	problem, _ := sampleProblem(t)

	plans, _ := planning.Search(problem, allObjectives())

	for _, plan := range plans {
		if plan.WorstCasePeak < plan.ExpectedPeak {
			t.Fatalf("%s kasus terburuk lebih rendah dari kasus harapan: %v < %v",
				plan.Objective, plan.WorstCasePeak, plan.ExpectedPeak)
		}
	}
}

func TestCoverageNeverExceedsTheDemandAsked(t *testing.T) {
	problem, _ := sampleProblem(t)

	plans, _ := planning.Search(problem, allObjectives())

	for _, plan := range plans {
		if plan.Metrics.CoverageKg > 5000 {
			t.Fatalf("%s menutup lebih banyak daripada yang diminta: %d", plan.Objective, plan.Metrics.CoverageKg)
		}
	}
}

func TestGrossValueIsNilWhenAChosenCandidateHasNoPrice(t *testing.T) {
	input := sampleInput(t)
	input.PricePerKg = nil
	candidates := planning.BuildCandidates(input)

	problem := planning.NewProblem(candidates, nil, nil)
	plans, _ := planning.Search(problem, allObjectives())

	for _, plan := range plans {
		if plan.Metrics.GrossValue != nil {
			t.Fatalf("%s melaporkan nilai padahal komoditasnya tanpa harga acuan", plan.Objective)
		}
	}
}

func TestSearchOfAnEmptyProblemIsEmpty(t *testing.T) {
	plans, evaluations := planning.Search(planning.NewProblem(nil, nil, nil), allObjectives())

	if len(plans) != 3 || evaluations != 0 {
		t.Fatalf("soal kosong harus menghasilkan tiga rencana kosong tanpa evaluasi, dapat %d rencana %d evaluasi",
			len(plans), evaluations)
	}
	for _, plan := range plans {
		if len(plan.CandidateIDs) != 0 {
			t.Fatalf("%s tidak kosong", plan.Objective)
		}
	}
}

func TestWeekKeyIsAlwaysAMonday(t *testing.T) {
	for _, iso := range []string{"2027-01-01", "2027-01-03", "2027-01-04", "2026-12-31"} {
		key := planning.WeekKey(mustDate(t, iso))
		parsed := mustDate(t, key)
		if parsed.Weekday().String() != "Monday" {
			t.Fatalf("%s menghasilkan kunci %s yang bukan Senin", iso, key)
		}
	}
}
