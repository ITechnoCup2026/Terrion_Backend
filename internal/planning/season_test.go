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

	// Jendela tanam MT I panjangnya ~13 minggu, dan langkahnya dua minggu
	// (PlantingDateStepDays), jadi sekitar tujuh tanggal. Rentangnya dibiarkan
	// longgar karena jumlah persisnya bergeser menurut posisi `now` di dalam
	// jendela; yang diperiksa di sini bentuk tanggalnya, bukan cacahnya.
	if len(dates) < 5 || len(dates) > 9 {
		t.Fatalf("len(dates) = %d, want between 5 and 9", len(dates))
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

func TestPreviousSeasonComparesLikeWithLike(t *testing.T) {
	first := planning.SeasonMT1(2026)
	if got := planning.PreviousSeason(first); got.Label != "MT I 2025/2026" {
		t.Errorf("planning.PreviousSeason(%q) = %q, mau %q — MT I harus dibanding MT I",
			first.Label, got.Label, "MT I 2025/2026")
	}

	second := planning.SeasonMT2(2026)
	if got := planning.PreviousSeason(second); got.Label != "MT II 2025" {
		t.Errorf("planning.PreviousSeason(%q) = %q, mau %q",
			second.Label, got.Label, "MT II 2025")
	}
}

// Kandidat tanam melangkah dua minggu, bukan satu.
//
// Setiap tanggal tanam dikalikan jumlah lahan dan jumlah varietas, jadi jarak
// langkahnya menentukan besar ruang pencarian. Sejak varietas acuan bertambah
// dari 13 menjadi 42, langkah mingguan membuat ruang itu membengkak sampai
// melewati batas yang bisa diterima layanan AI maupun yang wajar dihitung
// solver lokal. Dua minggu memangkasnya separuh; seminggu lebih presisi
// daripada yang bisa dipegang siapa pun saat menanam.
func TestCandidatePlantingDatesStepFortnightly(t *testing.T) {
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)

	dates := planning.CandidatePlantingDates(planning.SeasonMT1(2026), now)

	if len(dates) < 2 {
		t.Fatalf("len(dates) = %d, butuh minimal 2 untuk memeriksa jaraknya", len(dates))
	}
	for i := 1; i < len(dates); i++ {
		if gap := agronomy.DaysBetween(dates[i-1], dates[i]); gap != 14 {
			t.Errorf("jarak %s -> %s = %d hari, mau 14",
				agronomy.ToISODate(dates[i-1]), agronomy.ToISODate(dates[i]), gap)
		}
	}
}
