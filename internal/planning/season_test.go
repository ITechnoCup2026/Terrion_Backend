package planning_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/planning"
)

func TestSeasonMT1SpansOctoberToMarch(t *testing.T) {
	season := planning.SeasonMT1(2026)

	if season.Label != "MT I 2026/2027" {
		t.Errorf("Label = %q, want \"MT I 2026/2027\"", season.Label)
	}
	if got := agronomy.ToISODate(season.Start); got != "2026-10-01" {
		t.Errorf("Start = %s, want 2026-10-01", got)
	}
	if got := agronomy.ToISODate(season.End); got != "2027-03-31" {
		t.Errorf("End = %s, want 2027-03-31", got)
	}
	if got := agronomy.ToISODate(season.PlantingTo); got != "2026-12-31" {
		t.Errorf("PlantingTo = %s, want 2026-12-31", got)
	}
}

func TestSeasonMT2SpansAprilToSeptember(t *testing.T) {
	season := planning.SeasonMT2(2027)

	if season.Label != "MT II 2027" {
		t.Errorf("Label = %q, want \"MT II 2027\"", season.Label)
	}
	if got := agronomy.ToISODate(season.Start); got != "2027-04-01" {
		t.Errorf("Start = %s, want 2027-04-01", got)
	}
	if got := agronomy.ToISODate(season.PlantingTo); got != "2027-06-30" {
		t.Errorf("PlantingTo = %s, want 2027-06-30", got)
	}
}

func TestOpenSeasonsFromSeptemberOffersMT1First(t *testing.T) {
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)

	seasons := planning.OpenSeasons(now)

	if len(seasons) == 0 {
		t.Fatal("OpenSeasons returned nothing")
	}
	if seasons[0].Label != "MT I 2026/2027" {
		t.Errorf("first season = %q, want \"MT I 2026/2027\"", seasons[0].Label)
	}
	for _, season := range seasons {
		if !season.PlantingTo.After(now) {
			t.Errorf("season %q has a closed planting window", season.Label)
		}
	}
}

func TestCandidatePlantingDatesAreWeekStartsInsideTheWindow(t *testing.T) {
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	season := planning.SeasonMT1(2026)

	dates := planning.CandidatePlantingDates(season, now)

	if len(dates) < 10 || len(dates) > 16 {
		t.Fatalf("len(dates) = %d, want between 10 and 16", len(dates))
	}
	for _, date := range dates {
		if date.Weekday() != time.Monday {
			t.Errorf("%s is a %s, want Monday", agronomy.ToISODate(date), date.Weekday())
		}
		if !date.After(now) {
			t.Errorf("%s is not after now", agronomy.ToISODate(date))
		}
		if date.Before(season.PlantingFrom) || date.After(season.PlantingTo) {
			t.Errorf("%s falls outside the planting window", agronomy.ToISODate(date))
		}
	}
}

func TestCandidatePlantingDatesAreEmptyOnceTheWindowClosed(t *testing.T) {
	now := time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)

	dates := planning.CandidatePlantingDates(planning.SeasonMT1(2026), now)

	if len(dates) != 0 {
		t.Errorf("len(dates) = %d, want 0", len(dates))
	}
}

func TestSeasonByLabelRoundTrips(t *testing.T) {
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)

	season, found := planning.SeasonByLabel("MT I 2026/2027", now)
	if !found {
		t.Fatal("SeasonByLabel did not find \"MT I 2026/2027\"")
	}
	if got := agronomy.ToISODate(season.Start); got != "2026-10-01" {
		t.Errorf("Start = %s, want 2026-10-01", got)
	}

	if _, found := planning.SeasonByLabel("MT III 2026", now); found {
		t.Error("SeasonByLabel accepted a label that does not exist")
	}
}
