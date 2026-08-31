package agronomy_test

import (
	"testing"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

const staggerToday = "2026-08-27"

func plantedBlock(t *testing.T, id, plantingDate string) agronomy.ShiftCandidate {
	t.Helper()
	return agronomy.ShiftCandidate{BlockID: id, PlantingDate: mustDate(t, plantingDate)}
}

func TestPlanStaggerShiftsABlockNotYetPlanted(t *testing.T) {
	plan := agronomy.PlanStagger(agronomy.StaggerPlanInput{
		BlockIDs:  []string{"b1"},
		ShiftDays: 10,
		Blocks:    []agronomy.ShiftCandidate{plantedBlock(t, "b1", "2026-09-10")},
		Today:     mustDate(t, staggerToday),
	})

	if len(plan.Refused) != 0 {
		t.Errorf("Refused = %+v, want none", plan.Refused)
	}
	if len(plan.Shifts) != 1 {
		t.Fatalf("len(Shifts) = %d, want 1", len(plan.Shifts))
	}
	shift := plan.Shifts[0]
	if shift.BlockID != "b1" {
		t.Errorf("BlockID = %q, want b1", shift.BlockID)
	}
	if !shift.OriginalDate.Equal(mustDate(t, "2026-09-10")) {
		t.Errorf("OriginalDate = %v, want 2026-09-10", shift.OriginalDate)
	}
	if !shift.ShiftedDate.Equal(mustDate(t, "2026-09-20")) {
		t.Errorf("ShiftedDate = %v, want 2026-09-20", shift.ShiftedDate)
	}
}

func TestPlanStaggerRefusals(t *testing.T) {
	tests := []struct {
		name         string
		plantingDate string
		shiftDays    int
		want         constants.RefusalReason
	}{
		{"already in the ground", "2026-06-25", 10, constants.RefusedAlreadyPlanted},
		{"planted today", staggerToday, 7, constants.RefusedAlreadyPlanted},
		{"negative shift lands in the past", "2026-09-02", -14, constants.RefusedWouldBeInPast},
		{"zero shift moves nothing", "2026-10-01", 0, constants.RefusedNoShift},
	}

	for _, test := range tests {
		plan := agronomy.PlanStagger(agronomy.StaggerPlanInput{
			BlockIDs:  []string{"b1"},
			ShiftDays: test.shiftDays,
			Blocks:    []agronomy.ShiftCandidate{plantedBlock(t, "b1", test.plantingDate)},
			Today:     mustDate(t, staggerToday),
		})

		if len(plan.Shifts) != 0 {
			t.Errorf("%s: Shifts = %+v, want none", test.name, plan.Shifts)
		}
		if len(plan.Refused) != 1 || plan.Refused[0].Reason != test.want {
			t.Errorf("%s: Refused = %+v, want one %q", test.name, plan.Refused, test.want)
		}
	}
}

func TestPlanStaggerAllowsANegativeShiftThatStaysInTheFuture(t *testing.T) {
	plan := agronomy.PlanStagger(agronomy.StaggerPlanInput{
		BlockIDs:  []string{"b1"},
		ShiftDays: -7,
		Blocks:    []agronomy.ShiftCandidate{plantedBlock(t, "b1", "2026-09-20")},
		Today:     mustDate(t, staggerToday),
	})

	if len(plan.Refused) != 0 {
		t.Errorf("Refused = %+v, want none", plan.Refused)
	}
	if len(plan.Shifts) != 1 {
		t.Fatalf("len(Shifts) = %d, want 1", len(plan.Shifts))
	}
	if !plan.Shifts[0].ShiftedDate.Equal(mustDate(t, "2026-09-13")) {
		t.Errorf("ShiftedDate = %v, want 2026-09-13", plan.Shifts[0].ShiftedDate)
	}
}

func TestPlanStaggerMovesWhatItCanAndReportsWhatItCannot(t *testing.T) {
	plan := agronomy.PlanStagger(agronomy.StaggerPlanInput{
		BlockIDs:  []string{"past", "future"},
		ShiftDays: 10,
		Blocks: []agronomy.ShiftCandidate{
			plantedBlock(t, "past", "2026-05-01"),
			plantedBlock(t, "future", "2026-10-01"),
		},
		Today: mustDate(t, staggerToday),
	})

	if len(plan.Shifts) != 1 || plan.Shifts[0].BlockID != "future" {
		t.Errorf("Shifts = %+v, want only future", plan.Shifts)
	}
	if len(plan.Refused) != 1 || plan.Refused[0].BlockID != "past" ||
		plan.Refused[0].Reason != constants.RefusedAlreadyPlanted {
		t.Errorf("Refused = %+v, want past already-planted", plan.Refused)
	}
}

func TestPlanStaggerIgnoresBlockIDsTheCooperativeDoesNotOwn(t *testing.T) {
	plan := agronomy.PlanStagger(agronomy.StaggerPlanInput{
		BlockIDs:  []string{"b1", "not-ours"},
		ShiftDays: 10,
		Blocks:    []agronomy.ShiftCandidate{plantedBlock(t, "b1", "2026-10-01")},
		Today:     mustDate(t, staggerToday),
	})

	if len(plan.Shifts) != 1 || plan.Shifts[0].BlockID != "b1" {
		t.Errorf("Shifts = %+v, want only b1", plan.Shifts)
	}
	if len(plan.Refused) != 0 {
		t.Errorf("Refused = %+v, want none", plan.Refused)
	}
}

func TestPlanStaggerOfAnEmptySuggestionDoesNothing(t *testing.T) {
	plan := agronomy.PlanStagger(agronomy.StaggerPlanInput{
		ShiftDays: 10,
		Blocks:    []agronomy.ShiftCandidate{plantedBlock(t, "b1", "2026-10-01")},
		Today:     mustDate(t, staggerToday),
	})

	if len(plan.Shifts) != 0 || len(plan.Refused) != 0 {
		t.Errorf("plan = %+v, want empty", plan)
	}
}
