package agronomy_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
)

func panelWeek(t *testing.T, commodityID, iso string, price float64) agronomy.ReferencePrice {
	t.Helper()
	return agronomy.ReferencePrice{
		CommodityID: commodityID,
		WeekStart:   mustDate(t, iso),
		PricePerKg:  price,
	}
}

// 2025-03-03 is the Monday of the same ISO week as 2026-03-02.
func panel(t *testing.T) []agronomy.ReferencePrice {
	t.Helper()
	return []agronomy.ReferencePrice{
		panelWeek(t, "padi", "2025-03-03", 6200),
		panelWeek(t, "padi", "2026-08-31", 7100),
		panelWeek(t, "padi", "2026-08-24", 7000),
		panelWeek(t, "jagung", "2026-08-31", 4800),
	}
}

func TestBenchmarkForTakesTheNewestWeekAsLatest(t *testing.T) {
	found := agronomy.BenchmarkFor(panel(t), "padi", mustDate(t, "2026-03-02"))

	if found == nil {
		t.Fatal("expected a benchmark for padi")
	}
	if found.Latest.PricePerKg != 7100 {
		t.Fatalf("latest = %v, want 7100", found.Latest.PricePerKg)
	}
}

func TestBenchmarkForMatchesTheWindowWeekAYearBack(t *testing.T) {
	found := agronomy.BenchmarkFor(panel(t), "padi", mustDate(t, "2026-03-02"))

	if found == nil || found.Seasonal == nil {
		t.Fatal("expected a seasonal price for the window week")
	}
	if found.Seasonal.PricePerKg != 6200 {
		t.Fatalf("seasonal = %v, want 6200", found.Seasonal.PricePerKg)
	}
}

func TestBenchmarkForLeavesSeasonalNilOutsideThePanel(t *testing.T) {
	// A window in 2030 has no week a year back that the panel published.
	found := agronomy.BenchmarkFor(panel(t), "padi", mustDate(t, "2030-03-04"))

	if found == nil {
		t.Fatal("expected a benchmark, latest is still known")
	}
	if found.Seasonal != nil {
		t.Fatalf("seasonal = %v, want nil", *found.Seasonal)
	}
}

func TestBenchmarkForLeavesSeasonalNilWithoutAWindow(t *testing.T) {
	found := agronomy.BenchmarkFor(panel(t), "padi", time.Time{})

	if found == nil || found.Seasonal != nil {
		t.Fatal("a block with no window has a latest price and no seasonal one")
	}
}

func TestBenchmarkForIgnoresOtherCommodities(t *testing.T) {
	found := agronomy.BenchmarkFor(panel(t), "jagung", mustDate(t, "2026-03-02"))

	if found == nil || found.Latest.PricePerKg != 4800 {
		t.Fatalf("jagung latest = %v, want 4800", found)
	}
	if found.Seasonal != nil {
		t.Fatal("the panel has no jagung week a year before the window")
	}
}

func TestBenchmarkForIsNilForAnUnpricedCommodity(t *testing.T) {
	if found := agronomy.BenchmarkFor(panel(t), "cabai", mustDate(t, "2026-03-02")); found != nil {
		t.Fatalf("cabai = %v, want nil", found)
	}
}
