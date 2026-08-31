package agronomy_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
)

func mustDate(t *testing.T, iso string) time.Time {
	t.Helper()
	parsed, err := agronomy.UTCDate(iso)
	if err != nil {
		t.Fatalf("UTCDate(%q): %v", iso, err)
	}
	return parsed
}

func TestUTCDateHasNoLocalTimeDrift(t *testing.T) {
	if got := agronomy.ToISODate(mustDate(t, "2026-10-14")); got != "2026-10-14" {
		t.Errorf("round trip = %q, want 2026-10-14", got)
	}
}

func TestUTCDateRejectsMalformedInput(t *testing.T) {
	if _, err := agronomy.UTCDate("14-10-2026"); err == nil {
		t.Error("UTCDate(\"14-10-2026\") = nil error, want a parse failure")
	}
}

func TestAddDaysCrossesMonthBoundary(t *testing.T) {
	got := agronomy.ToISODate(agronomy.AddDays(mustDate(t, "2026-10-30"), 5))
	if got != "2026-11-04" {
		t.Errorf("2026-10-30 + 5 days = %q, want 2026-11-04", got)
	}
}

func TestDaysBetween(t *testing.T) {
	got := agronomy.DaysBetween(mustDate(t, "2026-10-10"), mustDate(t, "2026-10-17"))
	if got != 7 {
		t.Errorf("DaysBetween = %d, want 7", got)
	}
}

func TestDaysBetweenIsNegativeWhenReversed(t *testing.T) {
	got := agronomy.DaysBetween(mustDate(t, "2026-10-17"), mustDate(t, "2026-10-10"))
	if got != -7 {
		t.Errorf("DaysBetween = %d, want -7", got)
	}
}

func TestDayOfYear(t *testing.T) {
	tests := []struct {
		iso  string
		want int
	}{
		{"2026-01-01", 1},
		{"2026-12-31", 365},
	}
	for _, test := range tests {
		if got := agronomy.DayOfYear(mustDate(t, test.iso)); got != test.want {
			t.Errorf("DayOfYear(%s) = %d, want %d", test.iso, got, test.want)
		}
	}
}

func TestISOWeekKey(t *testing.T) {
	tests := []struct {
		iso  string
		want string
	}{
		{"2026-01-01", "2026-W01"},
		{"2026-10-14", "2026-W42"},
		{"2027-01-01", "2026-W53"},
	}
	for _, test := range tests {
		if got := agronomy.ISOWeekKey(mustDate(t, test.iso)); got != test.want {
			t.Errorf("ISOWeekKey(%s) = %q, want %q", test.iso, got, test.want)
		}
	}
}

func TestISOWeekStartIsMonday(t *testing.T) {
	got := agronomy.ToISODate(agronomy.ISOWeekStart(mustDate(t, "2026-10-14")))
	if got != "2026-10-12" {
		t.Errorf("ISOWeekStart = %q, want 2026-10-12", got)
	}
}

func TestISOWeekStartOfSundayStaysInTheWeekJustEnded(t *testing.T) {
	got := agronomy.ToISODate(agronomy.ISOWeekStart(mustDate(t, "2026-10-18")))
	if got != "2026-10-12" {
		t.Errorf("ISOWeekStart(Sunday) = %q, want 2026-10-12", got)
	}
}

func TestISOWeekStartDiscardsTimeOfDay(t *testing.T) {
	afternoon := time.Date(2026, 10, 14, 15, 30, 0, 0, time.UTC)
	if got := agronomy.ToISODate(agronomy.ISOWeekStart(afternoon)); got != "2026-10-12" {
		t.Errorf("ISOWeekStart(afternoon) = %q, want 2026-10-12", got)
	}
}

func TestStartOfDayNormalisesAwayFromLocalTime(t *testing.T) {
	jakarta := time.FixedZone("WIB", 7*60*60)
	earlyMorning := time.Date(2026, 10, 15, 3, 0, 0, 0, jakarta)

	if got := agronomy.ToISODate(agronomy.StartOfDay(earlyMorning)); got != "2026-10-14" {
		t.Errorf("StartOfDay(03:00 WIB on 15 Oct) = %q, want 2026-10-14", got)
	}
}
