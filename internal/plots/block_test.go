package plots_test

import (
	"math"
	"testing"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/plots"
)

func TestBlockLabelWalksTheAlphabetThenNumbers(t *testing.T) {
	tests := []struct {
		orderIndex int
		want       string
	}{
		{0, "BLOK A"},
		{1, "BLOK B"},
		{25, "BLOK Z"},
		{26, "BLOK 27"},
	}

	for _, test := range tests {
		if got := plots.BlockLabel(test.orderIndex); got != test.want {
			t.Errorf("BlockLabel(%d) = %q, want %q", test.orderIndex, got, test.want)
		}
	}
}

func TestPlotAreaHaSumsToTheStoredPrecision(t *testing.T) {
	got := plots.PlotAreaHa([]float64{0.3, 0.42})

	if got != 0.72 {
		t.Errorf("PlotAreaHa = %v, want 0.72 without floating point dust", got)
	}
}

func TestPlotAreaHaOfNothingIsZero(t *testing.T) {
	if got := plots.PlotAreaHa(nil); got != 0 {
		t.Errorf("PlotAreaHa(nil) = %v, want 0", got)
	}
}

func TestPlanSplitDividesABlock(t *testing.T) {
	plan, refusal := plots.PlanSplit(1.2, 0.4)

	if refusal != nil {
		t.Fatalf("PlanSplit refused with %+v, want a plan", refusal)
	}
	if plan.TakenHa != 0.4 {
		t.Errorf("TakenHa = %v, want 0.4", plan.TakenHa)
	}
	if plan.KeptHa != 0.8 {
		t.Errorf("KeptHa = %v, want 0.8", plan.KeptHa)
	}
}

func TestPlanSplitHalvesStillSumToTheOriginal(t *testing.T) {
	plan, refusal := plots.PlanSplit(0.72, 0.3)

	if refusal != nil {
		t.Fatalf("PlanSplit refused with %+v, want a plan", refusal)
	}
	if math.Abs(plan.KeptHa+plan.TakenHa-0.72) > 1e-9 {
		t.Errorf("kept %v + taken %v does not sum to 0.72", plan.KeptHa, plan.TakenHa)
	}
}

func TestPlanSplitRefusesATakeBelowTheMinimum(t *testing.T) {
	_, refusal := plots.PlanSplit(1.2, 0.004)

	if refusal == nil {
		t.Fatal("PlanSplit returned a plan, want a refusal")
	}
	if refusal.Code != constants.SplitBelowMinimum {
		t.Errorf("Code = %q, want %q", refusal.Code, constants.SplitBelowMinimum)
	}
	if refusal.MinHa != constants.MinPlantingHa {
		t.Errorf("MinHa = %v, want %v", refusal.MinHa, constants.MinPlantingHa)
	}
}

func TestPlanSplitRefusesWhenTooLittleWouldRemain(t *testing.T) {
	_, refusal := plots.PlanSplit(0.5, 0.495)

	if refusal == nil {
		t.Fatal("PlanSplit returned a plan, want a refusal")
	}
	if refusal.Code != constants.SplitLeavesTooLittle {
		t.Errorf("Code = %q, want %q", refusal.Code, constants.SplitLeavesTooLittle)
	}
	if refusal.BlockAreaHa != 0.5 {
		t.Errorf("BlockAreaHa = %v, want 0.5", refusal.BlockAreaHa)
	}
	if refusal.MaxTakeableHa != 0.49 {
		t.Errorf("MaxTakeableHa = %v, want 0.49", refusal.MaxTakeableHa)
	}
}

func TestPlanSplitRefusesTakingTheWholeBlock(t *testing.T) {
	_, refusal := plots.PlanSplit(0.5, 0.5)

	if refusal == nil || refusal.Code != constants.SplitLeavesTooLittle {
		t.Errorf("refusal = %+v, want %q", refusal, constants.SplitLeavesTooLittle)
	}
}
