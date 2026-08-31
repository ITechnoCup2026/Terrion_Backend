package plots

import (
	"fmt"
	"math"

	"terrion-backend/internal/constants"
)

func BlockLabel(orderIndex int) string {
	if orderIndex < 26 {
		return fmt.Sprintf("BLOK %c", rune('A'+orderIndex))
	}
	return fmt.Sprintf("BLOK %d", orderIndex+1)
}

func RoundArea(hectares float64) float64 {
	scale := math.Pow(10, constants.AreaDecimals)
	return math.Round(hectares*scale) / scale
}

func PlotAreaHa(plantingAreas []float64) float64 {
	total := 0.0
	for _, area := range plantingAreas {
		total += area
	}
	return RoundArea(total)
}

type SplitPlan struct {
	KeptHa  float64
	TakenHa float64
}

type SplitRefusal struct {
	Code          string
	MinHa         float64
	BlockAreaHa   float64
	MaxTakeableHa float64
}

func PlanSplit(blockAreaHa, takenHa float64) (SplitPlan, *SplitRefusal) {
	taken := RoundArea(takenHa)
	kept := RoundArea(blockAreaHa - taken)

	if taken < constants.MinPlantingHa {
		return SplitPlan{}, &SplitRefusal{
			Code:  constants.SplitBelowMinimum,
			MinHa: constants.MinPlantingHa,
		}
	}

	if kept < constants.MinPlantingHa {
		return SplitPlan{}, &SplitRefusal{
			Code:          constants.SplitLeavesTooLittle,
			MinHa:         constants.MinPlantingHa,
			BlockAreaHa:   blockAreaHa,
			MaxTakeableHa: RoundArea(blockAreaHa - constants.MinPlantingHa),
		}
	}

	return SplitPlan{KeptHa: kept, TakenHa: taken}, nil
}

func (r *SplitRefusal) Error() string {
	return r.Code
}
