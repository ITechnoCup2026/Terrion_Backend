package agronomy

import "time"

// One week's published reference price.
type WeekPrice struct {
	PricePerKg float64
	WeekStart  time.Time
}

// What the price panel can honestly say about a crop still in the ground.
//
// The harvest window is in the future and the panel only publishes weeks that
// have already happened, so there is no price FOR that week and there will not
// be one until it arrives. Seasonal is the closest honest stand-in: the same
// calendar week a year earlier, which is what tells a farmer whether their
// window opens into a glut or a gap. Latest is where the market stands today,
// and the seasonal figure means nothing without it -- a window priced 8% above
// last year's same week is not good news if the whole market is up 20%.
type PriceBenchmark struct {
	Latest WeekPrice
	// Where the panel's numbers come from, taken from the latest row. Carried
	// all the way to the screen: the seeded panel is synthetic, and a price a
	// farmer might sell on must never appear without saying whose it is.
	Source string
	// Nil when the panel holds no week matching the window a year back: a
	// window further out than the published history reaches, or a commodity
	// added to the panel less than a year ago.
	Seasonal *WeekPrice
}

// Picks the newest published week for a commodity, and the week a year before
// its harvest window opens. Nil when the panel says nothing about the commodity.
func BenchmarkFor(
	prices []ReferencePrice, commodityID string, windowStart time.Time,
) *PriceBenchmark {
	var latest *WeekPrice
	var seasonal *WeekPrice
	source := ""

	// A window a year back, keyed the same way ComputeImpact keys its weeks so
	// the two read the same panel the same way.
	//
	// 52 weeks, not AddDate(-1, 0, 0). A calendar year is 365 days and a year
	// of weeks is 364, so shifting by the calendar lands one weekday early --
	// from a Monday onto the Sunday before, which ISO counts as the PREVIOUS
	// week. The panel publishes Mondays, so that misses by a whole week every
	// time. 364 keeps the weekday and stays within a day of the anniversary.
	seasonalWeek := ""
	if !windowStart.IsZero() {
		seasonalWeek = ISOWeekKey(windowStart.AddDate(0, 0, -364))
	}

	for _, price := range prices {
		if price.CommodityID != commodityID {
			continue
		}
		week := WeekPrice{PricePerKg: price.PricePerKg, WeekStart: price.WeekStart}

		if latest == nil || week.WeekStart.After(latest.WeekStart) {
			found := week
			latest = &found
			source = price.Source
		}
		if seasonalWeek != "" && ISOWeekKey(week.WeekStart) == seasonalWeek {
			found := week
			seasonal = &found
		}
	}

	if latest == nil {
		return nil
	}
	return &PriceBenchmark{Latest: *latest, Seasonal: seasonal, Source: source}
}
