package catalog_test

import (
	"math"
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/catalog"
	"terrion-backend/internal/constants"
)

const (
	coopID   = "11111111-1111-1111-1111-111111111111"
	padiID   = "22222222-2222-2222-2222-222222222222"
	jagungID = "33333333-3333-3333-3333-333333333333"
)

func date(t *testing.T, iso string) time.Time {
	t.Helper()
	parsed, err := agronomy.UTCDate(iso)
	if err != nil {
		t.Fatalf("UTCDate(%q): %v", iso, err)
	}
	return parsed
}

func projection(
	t *testing.T, blockID, commodityID, start, end string, tonnes float64,
) agronomy.BlockProjection {
	t.Helper()
	return agronomy.BlockProjection{
		BlockID:        blockID,
		PlotID:         "plot-" + blockID,
		CommodityID:    commodityID,
		Window:         agronomy.DateRange{Start: date(t, start), End: date(t, end)},
		ExpectedTonnes: tonnes,
	}
}

func source(projections []agronomy.BlockProjection) catalog.Source {
	return catalog.Source{
		CooperativeID:     coopID,
		CooperativeName:   "Koperasi Tani Subang Jaya",
		Province:          "Jawa Barat",
		District:          "Kabupaten Subang",
		Village:           "Pamanukan",
		Projections:       projections,
		VarietyByBlock:    map[string]string{},
		ClimatologyBlocks: map[string]bool{},
		CommodityNames:    map[string]string{padiID: "Padi", jagungID: "Jagung"},
	}
}

func listingFor(t *testing.T, listings []catalog.Listing, commodityID string) catalog.Listing {
	t.Helper()
	for _, listing := range listings {
		if listing.CommodityID == commodityID {
			return listing
		}
	}
	t.Fatalf("no listing for commodity %q", commodityID)
	return catalog.Listing{}
}

func TestListingIDRoundTrips(t *testing.T) {
	parsed, ok := catalog.ParseListingID(catalog.ListingID(coopID, padiID, "2026-W37"))

	if !ok {
		t.Fatal("ParseListingID rejected an id it just built")
	}
	if parsed.CooperativeID != coopID || parsed.CommodityID != padiID ||
		parsed.ISOWeek != "2026-W37" {
		t.Errorf("parsed = %+v, want the three parts back", parsed)
	}
}

func TestParseListingIDRejectsMalformedIDs(t *testing.T) {
	malformed := []string{
		"",
		"not-an-id",
		coopID + "--" + padiID,
		coopID + "--" + padiID + "--2026-W99x",
		"nope--" + padiID + "--2026-W37",
	}

	for _, raw := range malformed {
		if _, ok := catalog.ParseListingID(raw); ok {
			t.Errorf("ParseListingID(%q) accepted a malformed id", raw)
		}
	}
}

func TestBuildListingsMakesOnePerCommodityPerWeek(t *testing.T) {
	from := date(t, "2026-09-07")

	listings := catalog.BuildListings([]catalog.Source{source([]agronomy.BlockProjection{
		projection(t, "b1", padiID, "2026-09-07", "2026-09-13", 20),
		projection(t, "b2", padiID, "2026-09-07", "2026-09-13", 16.7),
		projection(t, "b3", jagungID, "2026-09-14", "2026-09-20", 8),
	})}, from, 0)

	if len(listings) != 2 {
		t.Fatalf("len(listings) = %d, want 2", len(listings))
	}
	if listings[0].CommodityName != "Padi" {
		t.Errorf("listings[0].CommodityName = %q, want Padi", listings[0].CommodityName)
	}
	if math.Abs(listings[0].Tonnes-36.7) > 5e-6 {
		t.Errorf("Tonnes = %v, want 36.7", listings[0].Tonnes)
	}
	if len(listings[0].BlockIDs) != 2 {
		t.Errorf("BlockIDs = %v, want both padi blocks", listings[0].BlockIDs)
	}
	if listings[1].CommodityName != "Jagung" {
		t.Errorf("listings[1].CommodityName = %q, want Jagung", listings[1].CommodityName)
	}
}

func TestBuildListingsSortsSoonestThenHeaviest(t *testing.T) {
	from := date(t, "2026-09-07")

	listings := catalog.BuildListings([]catalog.Source{source([]agronomy.BlockProjection{
		projection(t, "late", padiID, "2026-09-14", "2026-09-20", 50),
		projection(t, "small", jagungID, "2026-09-07", "2026-09-13", 5),
		projection(t, "big", padiID, "2026-09-07", "2026-09-13", 30),
	})}, from, 0)

	want := []string{"big", "small", "late"}
	for i, listing := range listings {
		if listing.BlockIDs[0] != want[i] {
			t.Errorf("listings[%d] leads with %q, want %q", i, listing.BlockIDs[0], want[i])
		}
	}
}

func TestBuildListingsNamesTheVarietyOnlyWhenThereIsExactlyOne(t *testing.T) {
	from := date(t, "2026-09-07")

	catalogSource := source([]agronomy.BlockProjection{
		projection(t, "b1", padiID, "2026-09-07", "2026-09-13", 10),
		projection(t, "b2", padiID, "2026-09-07", "2026-09-13", 10),
		projection(t, "b3", jagungID, "2026-09-07", "2026-09-13", 10),
	})
	catalogSource.VarietyByBlock = map[string]string{
		"b1": "Ciherang", "b2": "IR64", "b3": "Bisi-18",
	}

	listings := catalog.BuildListings([]catalog.Source{catalogSource}, from, 0)

	if listingFor(t, listings, padiID).VarietyName != nil {
		t.Error("padi listing names a variety, want nil when the week mixes two")
	}
	jagung := listingFor(t, listings, jagungID)
	if jagung.VarietyName == nil || *jagung.VarietyName != "Bisi-18" {
		t.Errorf("jagung VarietyName = %v, want Bisi-18", jagung.VarietyName)
	}
}

func TestBuildListingsMarksAWeekResitngOnClimatology(t *testing.T) {
	from := date(t, "2026-09-07")

	catalogSource := source([]agronomy.BlockProjection{
		projection(t, "firm", padiID, "2026-09-07", "2026-09-13", 10),
		projection(t, "guess", padiID, "2026-09-07", "2026-09-13", 10),
	})
	catalogSource.ClimatologyBlocks = map[string]bool{"guess": true}

	listings := catalog.BuildListings([]catalog.Source{catalogSource}, from, 0)

	if listings[0].Basis != constants.BasisClimatology {
		t.Errorf("Basis = %q, want %q", listings[0].Basis, constants.BasisClimatology)
	}
}

func TestBuildListingsDropsWeeksOutsideTheHorizon(t *testing.T) {
	from := date(t, "2026-09-07")

	listings := catalog.BuildListings([]catalog.Source{source([]agronomy.BlockProjection{
		projection(t, "past", padiID, "2026-08-24", "2026-08-30", 10),
		projection(t, "far", padiID, "2027-01-04", "2027-01-10", 10),
		projection(t, "inside", padiID, "2026-09-07", "2026-09-13", 10),
	})}, from, 12)

	if len(listings) != 1 || listings[0].BlockIDs[0] != "inside" {
		t.Errorf("listings = %+v, want only the one inside the horizon", listings)
	}
}

func catalogFixture(t *testing.T) ([]catalog.Listing, time.Time) {
	t.Helper()
	from := date(t, "2026-09-07")

	return catalog.BuildListings([]catalog.Source{source([]agronomy.BlockProjection{
		projection(t, "b1", padiID, "2026-09-07", "2026-09-13", 36.7),
		projection(t, "b2", jagungID, "2026-09-07", "2026-09-13", 4),
		projection(t, "b3", padiID, "2026-11-02", "2026-11-08", 20),
	})}, from, 0), from
}

func TestFilterListings(t *testing.T) {
	listings, from := catalogFixture(t)

	tests := []struct {
		name    string
		filters catalog.Filters
		want    int
	}{
		{"no filter", catalog.Filters{}, 3},
		{"by commodity", catalog.Filters{CommodityID: jagungID}, 1},
		{"by minimum tonnage", catalog.Filters{MinTonnes: 10}, 2},
		{"by weeks ahead", catalog.Filters{WeeksAhead: 4}, 2},
		{"combined", catalog.Filters{CommodityID: padiID, WeeksAhead: 4}, 1},
		{"by another province", catalog.Filters{Province: "Bali"}, 0},
		{"by this province", catalog.Filters{Province: "Jawa Barat"}, 3},
	}

	for _, test := range tests {
		if got := len(catalog.FilterListings(listings, test.filters, from)); got != test.want {
			t.Errorf("%s: %d listings, want %d", test.name, got, test.want)
		}
	}
}
