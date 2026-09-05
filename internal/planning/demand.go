package planning

import (
	"sort"
	"time"

	"terrion-backend/internal/agronomy"
)

const (
	seasonalShiftDays = 364
	maxSeasonalShifts = 10
)

type HistoricalRequest struct {
	CommodityID string
	VolumeKg    float64
	WindowStart time.Time
}

type Demand struct {
	CommodityID string
	ISOWeek     string
	WeekStart   time.Time
	Kg          float64
}

func DemandByWeek(requests []HistoricalRequest, season Season) []Demand {
	totals := map[string]*Demand{}
	keys := []string{}

	for _, request := range requests {
		shifted, reached := shiftIntoSeason(request.WindowStart, season)
		if !reached {
			continue
		}

		isoWeek := agronomy.ISOWeekKey(shifted)
		key := request.CommodityID + "|" + isoWeek

		held, seen := totals[key]
		if !seen {
			totals[key] = &Demand{
				CommodityID: request.CommodityID,
				ISOWeek:     isoWeek,
				WeekStart:   agronomy.ISOWeekStart(shifted),
				Kg:          request.VolumeKg,
			}
			keys = append(keys, key)
			continue
		}
		held.Kg += request.VolumeKg
	}

	sort.Strings(keys)

	demand := make([]Demand, len(keys))
	for i, key := range keys {
		demand[i] = *totals[key]
	}
	return demand
}

func shiftIntoSeason(windowStart time.Time, season Season) (time.Time, bool) {
	shifted := agronomy.StartOfDay(windowStart)

	for range maxSeasonalShifts {
		if !shifted.Before(season.Start) {
			break
		}
		shifted = agronomy.AddDays(shifted, seasonalShiftDays)
	}

	if shifted.Before(season.Start) || shifted.After(season.End) {
		return time.Time{}, false
	}
	return shifted, true
}
