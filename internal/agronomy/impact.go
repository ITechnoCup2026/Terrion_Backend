package agronomy

import (
	"time"

	"terrion-backend/internal/constants"
)

type HarvestedBlock struct {
	BlockID             string
	CommodityID         string
	ActualHarvestDate   *time.Time
	ActualYieldKg       *float64
	ActualPricePerKg    *float64
	PaymentReceivedDate *time.Time
}

type ReferencePrice struct {
	CommodityID string
	WeekStart   time.Time
	PricePerKg  float64
	Source      string
}

type OrderLine struct {
	Quantity           float64
	RetailPricePerUnit *float64
	BulkPricePerUnit   *float64
	Status             constants.OrderStatus
}

type StaggerRecord struct {
	SeasonLabel  string
	BlockID      string
	OriginalDate time.Time
	ShiftedDate  time.Time
}

type ImpactFigures struct {
	PriceVsReference *float64
	DaysToPayment    *float64
	InputCostSaved   *float64
	TonnesDiverted   *float64
}

type ImpactInput struct {
	Blocks          []HarvestedBlock
	ReferencePrices []ReferencePrice
	OrderLines      []OrderLine
	StaggerApplied  []StaggerRecord
	Projections     []BlockProjection
	Capacity        map[string]float64
}

func ComputeImpact(input ImpactInput) ImpactFigures {
	return ImpactFigures{
		PriceVsReference: priceVsReference(input.Blocks, input.ReferencePrices),
		DaysToPayment:    daysToPayment(input.Blocks),
		InputCostSaved:   inputCostSaved(input.OrderLines),
		TonnesDiverted:   tonnesDiverted(input.Projections, input.StaggerApplied, input.Capacity),
	}
}

func weekCell(commodityID string, day time.Time) string {
	return commodityID + "|" + ISOWeekKey(day)
}

func priceVsReference(blocks []HarvestedBlock, prices []ReferencePrice) *float64 {
	reference := make(map[string]float64, len(prices))
	for _, price := range prices {
		reference[weekCell(price.CommodityID, price.WeekStart)] = price.PricePerKg
	}

	weightedReceived := 0.0
	weightedReference := 0.0
	totalKg := 0.0

	for _, block := range blocks {
		if block.ActualHarvestDate == nil || block.ActualYieldKg == nil {
			continue
		}
		if block.ActualPricePerKg == nil {
			continue
		}
		referencePrice, covered := reference[weekCell(block.CommodityID, *block.ActualHarvestDate)]
		if !covered {
			continue
		}

		weightedReceived += *block.ActualPricePerKg * *block.ActualYieldKg
		weightedReference += referencePrice * *block.ActualYieldKg
		totalKg += *block.ActualYieldKg
	}

	if totalKg == 0 {
		return nil
	}
	return pointerTo((weightedReceived - weightedReference) / totalKg)
}

func daysToPayment(blocks []HarvestedBlock) *float64 {
	total := 0.0
	count := 0

	for _, block := range blocks {
		if block.ActualHarvestDate == nil || block.PaymentReceivedDate == nil {
			continue
		}
		total += float64(DaysBetween(*block.ActualHarvestDate, *block.PaymentReceivedDate))
		count++
	}

	if count == 0 {
		return nil
	}
	return pointerTo(total / float64(count))
}

func inputCostSaved(lines []OrderLine) *float64 {
	saved := 0.0
	count := 0

	for _, line := range lines {
		if line.Status != constants.OrderCompleted {
			continue
		}
		if line.RetailPricePerUnit == nil || line.BulkPricePerUnit == nil {
			continue
		}
		saved += line.Quantity * (*line.RetailPricePerUnit - *line.BulkPricePerUnit)
		count++
	}

	if count == 0 {
		return nil
	}
	return pointerTo(saved)
}

func tonnesDiverted(
	projections []BlockProjection, staggerApplied []StaggerRecord, capacity map[string]float64,
) *float64 {
	if len(staggerApplied) == 0 {
		return nil
	}

	shiftByBlock := make(map[string]int, len(staggerApplied))
	for _, record := range staggerApplied {
		shiftByBlock[record.BlockID] = DaysBetween(record.OriginalDate, record.ShiftedDate)
	}

	before := make([]BlockProjection, len(projections))
	for i, projection := range projections {
		if shift, moved := shiftByBlock[projection.BlockID]; moved {
			projection.Window = DateRange{
				Start: AddDays(projection.Window.Start, -shift),
				End:   AddDays(projection.Window.End, -shift),
			}
		}
		before[i] = projection
	}

	wouldHave := DetectCollisions(before, capacity)
	actual := DetectCollisions(projections, capacity)

	actualByWeek := make(map[string]float64, len(actual.Weeks))
	for _, week := range actual.Weeks {
		actualByWeek[week.CommodityID+"|"+week.ISOWeek] = week.Tonnes
	}

	diverted := 0.0
	for _, week := range wouldHave.Flagged {
		now := actualByWeek[week.CommodityID+"|"+week.ISOWeek]
		if left := week.Tonnes - now; left > 0 {
			diverted += left
		}
	}
	return pointerTo(diverted)
}

func pointerTo(value float64) *float64 {
	return &value
}
