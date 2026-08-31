package catalog

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

type Source struct {
	CooperativeID     string
	CooperativeName   string
	Province          string
	District          string
	Village           string
	Projections       []agronomy.BlockProjection
	VarietyByBlock    map[string]string
	ClimatologyBlocks map[string]bool
	CommodityNames    map[string]string
}

type Listing struct {
	ID              string                `json:"id"`
	CooperativeID   string                `json:"cooperative_id"`
	CooperativeName string                `json:"cooperative_name"`
	Province        string                `json:"province"`
	District        string                `json:"district"`
	Village         string                `json:"village"`
	CommodityID     string                `json:"commodity_id"`
	CommodityName   string                `json:"commodity_name"`
	VarietyName     *string               `json:"variety_name"`
	ISOWeek         string                `json:"iso_week"`
	WeekStart       time.Time             `json:"week_start"`
	WeekEnd         time.Time             `json:"week_end"`
	Tonnes          float64               `json:"tonnes"`
	BlockIDs        []string              `json:"block_ids"`
	Basis           constants.WindowBasis `json:"basis"`
}

type ParsedListingID struct {
	CooperativeID string
	CommodityID   string
	ISOWeek       string
}

type Filters struct {
	CommodityID string
	Province    string
	WeeksAhead  int
	MinTonnes   float64
}

var (
	uuidPattern    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	isoWeekPattern = regexp.MustCompile(`^\d{4}-W\d{2}$`)
)

func ListingID(cooperativeID, commodityID, isoWeek string) string {
	return strings.Join([]string{cooperativeID, commodityID, isoWeek},
		constants.ListingIDSeparator)
}

func ParseListingID(raw string) (ParsedListingID, bool) {
	parts := strings.Split(raw, constants.ListingIDSeparator)
	if len(parts) != 3 {
		return ParsedListingID{}, false
	}

	cooperativeID, commodityID, isoWeek := parts[0], parts[1], parts[2]
	if !uuidPattern.MatchString(strings.ToLower(cooperativeID)) {
		return ParsedListingID{}, false
	}
	if !uuidPattern.MatchString(strings.ToLower(commodityID)) {
		return ParsedListingID{}, false
	}
	if !isoWeekPattern.MatchString(isoWeek) {
		return ParsedListingID{}, false
	}

	return ParsedListingID{
		CooperativeID: cooperativeID,
		CommodityID:   commodityID,
		ISOWeek:       isoWeek,
	}, true
}

func BuildListings(sources []Source, from time.Time, weeks int) []Listing {
	if weeks <= 0 {
		weeks = constants.DefaultHorizonWeeks
	}

	first := agronomy.ISOWeekStart(from)
	horizon := map[string]bool{}
	for i := range weeks {
		horizon[agronomy.ISOWeekKey(agronomy.AddDays(first, i*7))] = true
	}

	listings := []Listing{}
	for _, source := range sources {
		for _, bucket := range agronomy.BucketByWeek(source.Projections) {
			if !horizon[bucket.ISOWeek] {
				continue
			}

			listings = append(listings, Listing{
				ID:              ListingID(source.CooperativeID, bucket.CommodityID, bucket.ISOWeek),
				CooperativeID:   source.CooperativeID,
				CooperativeName: source.CooperativeName,
				Province:        source.Province,
				District:        source.District,
				Village:         source.Village,
				CommodityID:     bucket.CommodityID,
				CommodityName:   commodityName(source, bucket.CommodityID),
				VarietyName:     soleVariety(source, bucket.BlockIDs),
				ISOWeek:         bucket.ISOWeek,
				WeekStart:       bucket.WeekStart,
				WeekEnd:         agronomy.AddDays(bucket.WeekStart, 6),
				Tonnes:          bucket.Tonnes,
				BlockIDs:        bucket.BlockIDs,
				Basis:           basisOf(source, bucket.BlockIDs),
			})
		}
	}

	sort.SliceStable(listings, func(i, j int) bool {
		if !listings[i].WeekStart.Equal(listings[j].WeekStart) {
			return listings[i].WeekStart.Before(listings[j].WeekStart)
		}
		return listings[i].Tonnes > listings[j].Tonnes
	})
	return listings
}

func FilterListings(listings []Listing, filters Filters, from time.Time) []Listing {
	var cutoff time.Time
	if filters.WeeksAhead > 0 {
		cutoff = agronomy.AddDays(agronomy.ISOWeekStart(from), filters.WeeksAhead*7-1)
	}

	kept := []Listing{}
	for _, listing := range listings {
		if filters.CommodityID != "" && listing.CommodityID != filters.CommodityID {
			continue
		}
		if filters.Province != "" && listing.Province != filters.Province {
			continue
		}
		if filters.MinTonnes > 0 && listing.Tonnes < filters.MinTonnes {
			continue
		}
		if !cutoff.IsZero() && listing.WeekStart.After(cutoff) {
			continue
		}
		kept = append(kept, listing)
	}
	return kept
}

func commodityName(source Source, commodityID string) string {
	if name, known := source.CommodityNames[commodityID]; known {
		return name
	}
	return constants.UnnamedCommodity
}

func soleVariety(source Source, blockIDs []string) *string {
	names := map[string]bool{}
	for _, blockID := range blockIDs {
		if name, known := source.VarietyByBlock[blockID]; known && name != "" {
			names[name] = true
		}
	}
	if len(names) != 1 {
		return nil
	}

	for name := range names {
		sole := name
		return &sole
	}
	return nil
}

func basisOf(source Source, blockIDs []string) constants.WindowBasis {
	for _, blockID := range blockIDs {
		if source.ClimatologyBlocks[blockID] {
			return constants.BasisClimatology
		}
	}
	return constants.BasisObserved
}
