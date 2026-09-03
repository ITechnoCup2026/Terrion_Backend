package constants

const (
	MinPlantingHa  = 0.01
	MaxPlotHa      = 1000.0
	MaxPlantings   = 6
	AreaDecimals   = 4
	PublicIDLength = 8
)

const (
	IndonesiaMinLat = -11.0
	IndonesiaMaxLat = 6.0
	IndonesiaMinLng = 95.0
	IndonesiaMaxLng = 141.0
)

const (
	SplitBelowMinimum     = "split_below_minimum"
	SplitLeavesTooLittle  = "split_leaves_too_little"
	SplitBlockAlreadyGone = "split_block_already_gone"
	SplitBlockHarvested   = "split_block_harvested"
)

// Why a harvest could not be recorded. Each is a fact about the block or the
// date rather than a validation message, so the browser can say something
// specific instead of "invalid input".
const (
	HarvestBlockAlreadyGone  = "harvest_block_already_gone"
	HarvestAlreadyRecorded   = "harvest_already_recorded"
	HarvestBeforePlanting    = "harvest_before_planting"
	HarvestInFuture          = "harvest_in_future"
	HarvestPaymentBeforeCrop = "harvest_payment_before_crop"
)

const (
	SubsidyCapHa = 2.0
	KgPerSack    = 50
)

const (
	RdkkSeasonDays     = 365
	RdkkDefaultLabel   = "musim ini"
	RdkkNothingToOrder = "rdkk_nothing_to_order"
)

const MemberWithoutName = "Anggota tanpa nama"
