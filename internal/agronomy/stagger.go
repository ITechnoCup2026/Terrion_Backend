package agronomy

import (
	"time"

	"terrion-backend/internal/constants"
)

type ShiftCandidate struct {
	BlockID      string
	PlantingDate time.Time
}

type PlannedShift struct {
	BlockID      string
	OriginalDate time.Time
	ShiftedDate  time.Time
}

type Refusal struct {
	BlockID string
	Reason  constants.RefusalReason
}

type StaggerPlan struct {
	Shifts  []PlannedShift
	Refused []Refusal
}

type StaggerPlanInput struct {
	BlockIDs  []string
	ShiftDays int
	Blocks    []ShiftCandidate
	Today     time.Time
}

func PlanStagger(input StaggerPlanInput) StaggerPlan {
	owned := make(map[string]ShiftCandidate, len(input.Blocks))
	for _, block := range input.Blocks {
		owned[block.BlockID] = block
	}

	plan := StaggerPlan{Shifts: []PlannedShift{}, Refused: []Refusal{}}

	for _, blockID := range input.BlockIDs {
		block, ours := owned[blockID]
		if !ours {
			continue
		}

		if input.ShiftDays == 0 {
			plan.Refused = append(plan.Refused, Refusal{blockID, constants.RefusedNoShift})
			continue
		}

		if DaysBetween(input.Today, block.PlantingDate) <= 0 {
			plan.Refused = append(plan.Refused, Refusal{blockID, constants.RefusedAlreadyPlanted})
			continue
		}

		shiftedDate := AddDays(block.PlantingDate, input.ShiftDays)
		if DaysBetween(input.Today, shiftedDate) <= 0 {
			plan.Refused = append(plan.Refused, Refusal{blockID, constants.RefusedWouldBeInPast})
			continue
		}

		plan.Shifts = append(plan.Shifts, PlannedShift{
			BlockID:      blockID,
			OriginalDate: block.PlantingDate,
			ShiftedDate:  shiftedDate,
		})
	}

	return plan
}
