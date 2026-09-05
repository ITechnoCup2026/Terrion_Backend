package constants

import "time"

const MedianCapacityMultiplier = 2.5

var StaggerShiftCandidateDays = [...]int{7, 10, 14, -7, -10, -14}

type ThresholdBasis string

const (
	ThresholdCapacity ThresholdBasis = "capacity"
	ThresholdMedian   ThresholdBasis = "median"
)

type RefusalReason string

const (
	RefusedAlreadyPlanted RefusalReason = "already-planted"
	RefusedWouldBeInPast  RefusalReason = "would-be-in-the-past"
	RefusedNoShift        RefusalReason = "no-shift"
)

const (
	DefaultHorizonWeeks = 12
	MaxHorizonWeeks     = 52
	PileUpMinPlots      = 2
	UpcomingDays        = 7
	UpcomingLimit       = 5
)

const (
	ListingIDSeparator = "--"
	UnnamedCommodity   = "Komoditas"
	CatalogCacheKey    = "terrion:catalog:"
	CatalogCacheTTL    = time.Hour
)

type DeliveryPreference string

const (
	DeliverToBuyerWarehouse DeliveryPreference = "antar_ke_gudang"
	CollectAtCooperative    DeliveryPreference = "ambil_di_koperasi"
	DeliveryUndecided       DeliveryPreference = "belum_ditentukan"
)

var DeliveryPreferenceLabels = map[DeliveryPreference]string{
	DeliverToBuyerWarehouse: "Antar ke gudang pembeli",
	CollectAtCooperative:    "Ambil sendiri di koperasi",
	DeliveryUndecided:       "Belum ditentukan",
}

const (
	ListingGone        = "listing_gone"
	ListingUnknown     = "listing_unknown"
	RequestNotFound    = "request_not_found"
	AllocationExceeded = "allocation_exceeded"
	KgPerTonne         = 1000
)

const (
	StaggerSuggestionStale = "stagger_suggestion_stale"
	StaggerNothingToShift  = "stagger_nothing_to_shift"
	StaggerSeasonPrefix    = "MT-"
)
