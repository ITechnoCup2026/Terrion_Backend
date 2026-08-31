package rdkk

import (
	"fmt"
	"math"

	"terrion-backend/internal/constants"
)

type OrderLineDraft struct {
	Item       string
	Quantity   float64
	Unit       string
	QuantityKg float64
}

func ToOrderLines(totals []RequirementLine) []OrderLineDraft {
	unit := fmt.Sprintf("karung %d kg", constants.KgPerSack)

	drafts := []OrderLineDraft{}
	for _, line := range totals {
		if line.QuantityKg <= 0 {
			continue
		}
		drafts = append(drafts, OrderLineDraft{
			Item:       line.InputItem,
			Quantity:   math.Ceil(line.QuantityKg / constants.KgPerSack),
			Unit:       unit,
			QuantityKg: line.QuantityKg,
		})
	}
	return drafts
}
