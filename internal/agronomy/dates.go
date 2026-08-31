package agronomy

import (
	"fmt"
	"math"
	"regexp"
	"time"

	"terrion-backend/internal/constants"
)

func UTCDate(iso string) (time.Time, error) {
	parsed, err := time.ParseInLocation(constants.ISODateLayout, iso, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing date %q: %w", iso, err)
	}
	return parsed, nil
}

func ToISODate(t time.Time) string {
	return t.UTC().Format(constants.ISODateLayout)
}

func StartOfDay(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func AddDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}

func DaysBetween(from, to time.Time) int {
	return int(math.Round(to.Sub(from).Hours() / 24))
}

func DayOfYear(t time.Time) int {
	return t.UTC().YearDay()
}

func ISOWeekStart(t time.Time) time.Time {
	day := StartOfDay(t)
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return AddDays(day, 1-weekday)
}

func ISOWeekKey(t time.Time) string {
	year, week := t.UTC().ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

var isoWeekKeyPattern = regexp.MustCompile(`^\d{4}-W\d{2}$`)

func IsISOWeekKey(value string) bool {
	return isoWeekKeyPattern.MatchString(value)
}
