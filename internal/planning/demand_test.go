package planning_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/planning"
)

func TestDemandByWeekShiftsLastSeasonOntoTheTargetSeason(t *testing.T) {
	season := planning.SeasonMT1(2026)
	lastMarch := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)

	demand := planning.DemandByWeek([]planning.HistoricalRequest{
		{CommodityID: "padi", VolumeKg: 12000, WindowStart: lastMarch},
	}, season)

	if len(demand) != 1 {
		t.Fatalf("len(demand) = %d, want 1", len(demand))
	}
	if demand[0].Kg != 12000 {
		t.Errorf("Kg = %v, want 12000", demand[0].Kg)
	}

	shifted := agronomy.AddDays(lastMarch, 364)
	if demand[0].ISOWeek != agronomy.ISOWeekKey(shifted) {
		t.Errorf("ISOWeek = %q, want %q", demand[0].ISOWeek, agronomy.ISOWeekKey(shifted))
	}
	if shifted.Before(season.Start) || shifted.After(season.End) {
		t.Fatalf("fixture is wrong: %s falls outside the season", agronomy.ToISODate(shifted))
	}
}

func TestDemandByWeekSumsRequestsLandingInTheSameWeek(t *testing.T) {
	season := planning.SeasonMT1(2026)
	lastMarch := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)

	demand := planning.DemandByWeek([]planning.HistoricalRequest{
		{CommodityID: "padi", VolumeKg: 12000, WindowStart: lastMarch},
		{CommodityID: "padi", VolumeKg: 3000, WindowStart: agronomy.AddDays(lastMarch, 2)},
	}, season)

	if len(demand) != 1 {
		t.Fatalf("len(demand) = %d, want 1", len(demand))
	}
	if demand[0].Kg != 15000 {
		t.Errorf("Kg = %v, want 15000", demand[0].Kg)
	}
}

func TestDemandByWeekDropsRequestsThatCannotReachTheSeason(t *testing.T) {
	season := planning.SeasonMT1(2026)
	lastJuly := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	demand := planning.DemandByWeek([]planning.HistoricalRequest{
		{CommodityID: "padi", VolumeKg: 9000, WindowStart: lastJuly},
	}, season)

	if len(demand) != 0 {
		t.Errorf("len(demand) = %d, want 0", len(demand))
	}
}

func TestDemandByWeekIsOrderedDeterministically(t *testing.T) {
	season := planning.SeasonMT1(2026)
	base := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)

	requests := []planning.HistoricalRequest{
		{CommodityID: "padi", VolumeKg: 1000, WindowStart: agronomy.AddDays(base, 14)},
		{CommodityID: "jagung", VolumeKg: 1000, WindowStart: base},
		{CommodityID: "padi", VolumeKg: 1000, WindowStart: base},
	}

	first := planning.DemandByWeek(requests, season)
	second := planning.DemandByWeek(requests, season)

	if len(first) != 3 {
		t.Fatalf("len(first) = %d, want 3", len(first))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("entry %d differs: %+v vs %+v", i, first[i], second[i])
		}
	}
	if first[0].CommodityID != "jagung" {
		t.Errorf("first entry = %q, want jagung", first[0].CommodityID)
	}
}
