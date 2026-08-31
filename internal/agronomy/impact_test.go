package agronomy_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

func ptrFloat(value float64) *float64 { return &value }

func ptrDate(t *testing.T, iso string) *time.Time {
	t.Helper()
	parsed := mustDate(t, iso)
	return &parsed
}

func paidHarvest(t *testing.T) agronomy.HarvestedBlock {
	t.Helper()
	return agronomy.HarvestedBlock{
		BlockID:             "b1",
		CommodityID:         "padi",
		ActualHarvestDate:   ptrDate(t, "2026-03-02"),
		ActualYieldKg:       ptrFloat(1000),
		ActualPricePerKg:    ptrFloat(6800),
		PaymentReceivedDate: ptrDate(t, "2026-03-16"),
	}
}

func completedLine() agronomy.OrderLine {
	return agronomy.OrderLine{
		Quantity:           10,
		RetailPricePerUnit: ptrFloat(15000),
		BulkPricePerUnit:   ptrFloat(13000),
		Status:             constants.OrderCompleted,
	}
}

func marchReference(t *testing.T, pricePerKg float64) agronomy.ReferencePrice {
	t.Helper()
	return agronomy.ReferencePrice{
		CommodityID: "padi",
		WeekStart:   mustDate(t, "2026-03-02"),
		PricePerKg:  pricePerKg,
	}
}

func requireFigure(t *testing.T, name string, figure *float64) float64 {
	t.Helper()
	if figure == nil {
		t.Fatalf("%s = nil, want a number", name)
	}
	return *figure
}

func requireNoFigure(t *testing.T, name string, figure *float64) {
	t.Helper()
	if figure != nil {
		t.Errorf("%s = %v, want nil", name, *figure)
	}
}

func TestComputeImpactWithNothingRecordedIsAllNil(t *testing.T) {
	figures := agronomy.ComputeImpact(agronomy.ImpactInput{})

	requireNoFigure(t, "PriceVsReference", figures.PriceVsReference)
	requireNoFigure(t, "DaysToPayment", figures.DaysToPayment)
	requireNoFigure(t, "InputCostSaved", figures.InputCostSaved)
	requireNoFigure(t, "TonnesDiverted", figures.TonnesDiverted)
}

func TestPriceVsReferenceReportsTheGapOverTheHarvestWeek(t *testing.T) {
	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks:          []agronomy.HarvestedBlock{paidHarvest(t)},
		ReferencePrices: []agronomy.ReferencePrice{marchReference(t, 6500)},
	})

	closeTo(t, "PriceVsReference",
		requireFigure(t, "PriceVsReference", figures.PriceVsReference), 300, 5e-6)
}

func TestPriceVsReferenceWeightsByTonnage(t *testing.T) {
	big := paidHarvest(t)
	big.BlockID = "big"
	big.ActualYieldKg = ptrFloat(9000)
	big.ActualPricePerKg = ptrFloat(6900)

	small := paidHarvest(t)
	small.BlockID = "small"
	small.ActualYieldKg = ptrFloat(1000)
	small.ActualPricePerKg = ptrFloat(6100)

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks:          []agronomy.HarvestedBlock{big, small},
		ReferencePrices: []agronomy.ReferencePrice{marchReference(t, 6500)},
	})

	closeTo(t, "PriceVsReference",
		requireFigure(t, "PriceVsReference", figures.PriceVsReference), 320, 5e-6)
}

func TestPriceVsReferenceIsNilWhenNoReferenceCoversTheHarvestWeek(t *testing.T) {
	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks: []agronomy.HarvestedBlock{paidHarvest(t)},
		ReferencePrices: []agronomy.ReferencePrice{{
			CommodityID: "padi",
			WeekStart:   mustDate(t, "2025-01-06"),
			PricePerKg:  6500,
		}},
	})

	requireNoFigure(t, "PriceVsReference", figures.PriceVsReference)
}

func TestPriceVsReferenceIgnoresHarvestedButUnpricedBlocks(t *testing.T) {
	unpriced := paidHarvest(t)
	unpriced.BlockID = "unpriced"
	unpriced.ActualPricePerKg = nil

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks:          []agronomy.HarvestedBlock{paidHarvest(t), unpriced},
		ReferencePrices: []agronomy.ReferencePrice{marchReference(t, 6500)},
	})

	closeTo(t, "PriceVsReference",
		requireFigure(t, "PriceVsReference", figures.PriceVsReference), 300, 5e-6)
}

func TestPriceVsReferenceReportsARealZeroWhenTheReferenceWasMatched(t *testing.T) {
	matched := paidHarvest(t)
	matched.ActualPricePerKg = ptrFloat(6500)

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks:          []agronomy.HarvestedBlock{matched},
		ReferencePrices: []agronomy.ReferencePrice{marchReference(t, 6500)},
	})

	if got := requireFigure(t, "PriceVsReference", figures.PriceVsReference); got != 0 {
		t.Errorf("PriceVsReference = %v, want 0", got)
	}
}

func TestDaysToPaymentAveragesAcrossPaidBlocks(t *testing.T) {
	first := paidHarvest(t)
	first.BlockID = "a"
	second := paidHarvest(t)
	second.BlockID = "b"
	second.PaymentReceivedDate = ptrDate(t, "2026-03-22")

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks: []agronomy.HarvestedBlock{first, second},
	})

	closeTo(t, "DaysToPayment",
		requireFigure(t, "DaysToPayment", figures.DaysToPayment), 17, 5e-6)
}

func TestDaysToPaymentIgnoresBlocksStillWaitingToBePaid(t *testing.T) {
	unpaid := paidHarvest(t)
	unpaid.BlockID = "unpaid"
	unpaid.PaymentReceivedDate = nil

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks: []agronomy.HarvestedBlock{paidHarvest(t), unpaid},
	})

	closeTo(t, "DaysToPayment",
		requireFigure(t, "DaysToPayment", figures.DaysToPayment), 14, 5e-6)
}

func TestDaysToPaymentIsNilWhenNothingHasBeenPaid(t *testing.T) {
	unpaid := paidHarvest(t)
	unpaid.PaymentReceivedDate = nil

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks: []agronomy.HarvestedBlock{unpaid},
	})

	requireNoFigure(t, "DaysToPayment", figures.DaysToPayment)
}

func TestInputCostSavedSumsQuantityTimesTheRetailBulkGap(t *testing.T) {
	second := completedLine()
	second.Quantity = 4
	second.RetailPricePerUnit = ptrFloat(9000)
	second.BulkPricePerUnit = ptrFloat(8500)

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		OrderLines: []agronomy.OrderLine{completedLine(), second},
	})

	closeTo(t, "InputCostSaved",
		requireFigure(t, "InputCostSaved", figures.InputCostSaved), 22000, 5e-6)
}

func TestInputCostSavedCountsCompletedOrdersOnly(t *testing.T) {
	draft := completedLine()
	draft.Status = constants.OrderDraft
	submitted := completedLine()
	submitted.Status = constants.OrderSubmitted

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		OrderLines: []agronomy.OrderLine{completedLine(), draft, submitted},
	})

	closeTo(t, "InputCostSaved",
		requireFigure(t, "InputCostSaved", figures.InputCostSaved), 20000, 5e-6)
}

func TestInputCostSavedReportsARealZeroWhenBulkSavedNothing(t *testing.T) {
	line := completedLine()
	line.RetailPricePerUnit = ptrFloat(13000)

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		OrderLines: []agronomy.OrderLine{line},
	})

	if got := requireFigure(t, "InputCostSaved", figures.InputCostSaved); got != 0 {
		t.Errorf("InputCostSaved = %v, want 0", got)
	}
}

func TestInputCostSavedIsNilWhenNoOrderHasCompleted(t *testing.T) {
	draft := completedLine()
	draft.Status = constants.OrderDraft

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		OrderLines: []agronomy.OrderLine{draft},
	})

	requireNoFigure(t, "InputCostSaved", figures.InputCostSaved)
}

func TestInputCostSavedIgnoresLinesMissingAPrice(t *testing.T) {
	noRetail := completedLine()
	noRetail.RetailPricePerUnit = nil
	noBulk := completedLine()
	noBulk.BulkPricePerUnit = nil

	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		OrderLines: []agronomy.OrderLine{completedLine(), noRetail, noBulk},
	})

	closeTo(t, "InputCostSaved",
		requireFigure(t, "InputCostSaved", figures.InputCostSaved), 20000, 5e-6)
}

func harvestWeekProjection(t *testing.T, blockID, start string, tonnes float64) agronomy.BlockProjection {
	t.Helper()
	day := mustDate(t, start)
	return agronomy.BlockProjection{
		BlockID:        blockID,
		PlotID:         "p-" + blockID,
		CommodityID:    "padi",
		Window:         agronomy.DateRange{Start: day, End: day},
		ExpectedTonnes: tonnes,
	}
}

func staggerRecord(t *testing.T, blockID, originalDate, shiftedDate string) agronomy.StaggerRecord {
	t.Helper()
	return agronomy.StaggerRecord{
		SeasonLabel:  "2026",
		BlockID:      blockID,
		OriginalDate: mustDate(t, originalDate),
		ShiftedDate:  mustDate(t, shiftedDate),
	}
}

func TestTonnesDivertedIsNilWhenNoStaggeringWasAccepted(t *testing.T) {
	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Projections: []agronomy.BlockProjection{
			harvestWeekProjection(t, "a", "2026-09-07", 10),
			harvestWeekProjection(t, "b", "2026-09-07", 10),
		},
	})

	requireNoFigure(t, "TonnesDiverted", figures.TonnesDiverted)
}

func TestTonnesDivertedMeasuresWhatTheShiftTookOutOfTheFlaggedWeek(t *testing.T) {
	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Capacity: map[string]float64{"padi": 20},
		Projections: []agronomy.BlockProjection{
			harvestWeekProjection(t, "a", "2026-09-07", 10),
			harvestWeekProjection(t, "b", "2026-09-07", 10),
			harvestWeekProjection(t, "c", "2026-09-14", 10),
			harvestWeekProjection(t, "d", "2026-09-14", 10),
		},
		StaggerApplied: []agronomy.StaggerRecord{
			staggerRecord(t, "c", "2026-09-07", "2026-09-14"),
			staggerRecord(t, "d", "2026-09-07", "2026-09-14"),
		},
	})

	closeTo(t, "TonnesDiverted",
		requireFigure(t, "TonnesDiverted", figures.TonnesDiverted), 20, 5e-6)
}

func TestTonnesDivertedReportsARealZeroWhenNothingWasRelieved(t *testing.T) {
	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Capacity: map[string]float64{"padi": 100},
		Projections: []agronomy.BlockProjection{
			harvestWeekProjection(t, "a", "2026-09-07", 10),
			harvestWeekProjection(t, "b", "2026-09-14", 10),
		},
		StaggerApplied: []agronomy.StaggerRecord{
			staggerRecord(t, "b", "2026-09-07", "2026-09-14"),
		},
	})

	if got := requireFigure(t, "TonnesDiverted", figures.TonnesDiverted); got != 0 {
		t.Errorf("TonnesDiverted = %v, want 0", got)
	}
}

func TestTonnesDivertedIgnoresRecordsForBlocksWithoutAProjection(t *testing.T) {
	figures := agronomy.ComputeImpact(agronomy.ImpactInput{
		Capacity: map[string]float64{"padi": 20},
		Projections: []agronomy.BlockProjection{
			harvestWeekProjection(t, "a", "2026-09-07", 10),
			harvestWeekProjection(t, "b", "2026-09-07", 10),
		},
		StaggerApplied: []agronomy.StaggerRecord{
			staggerRecord(t, "deleted", "2026-09-07", "2026-09-14"),
		},
	})

	if got := requireFigure(t, "TonnesDiverted", figures.TonnesDiverted); got != 0 {
		t.Errorf("TonnesDiverted = %v, want 0", got)
	}
}
