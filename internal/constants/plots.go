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

// Kapasitas gudang koperasi.
const (
	// CapacityCommodityUnknown: baris menyebut komoditas yang tidak ada di
	// tabel acuan. Ditolak seluruhnya, karena kapasitas yang tersimpan pada id
	// yang salah ketik adalah ambang yang pengurus kira sudah ia atur.
	CapacityCommodityUnknown = "capacity_commodity_unknown"
)

// Menyunting dan menghapus apa yang sudah terdaftar.
const (
	// EditBlockAlreadyGone: blok tidak ada, atau bukan milik koperasi ini.
	// Sengaja tidak membedakan keduanya -- keberadaan lahan koperasi lain pun
	// bukan sesuatu yang perlu dibocorkan.
	EditBlockAlreadyGone = "edit_block_already_gone"
	// EditBlockHarvested: bloknya sudah punya catatan panen. Panen itu sudah
	// masuk ke kalibrasi model hasil koperasi, jadi menyunting bloknya membuat
	// catatan panen menggambarkan sesuatu yang tidak pernah ditanam.
	EditBlockHarvested = "edit_block_harvested"
	// DeletePlotAlreadyGone: lahan tidak ada, atau bukan milik koperasi ini.
	DeletePlotAlreadyGone = "delete_plot_already_gone"
	// DeletePlotHarvested: salah satu bloknya sudah dipanen. Menghapusnya
	// mengubah setiap proyeksi berikutnya tanpa ada yang bisa menjelaskannya.
	DeletePlotHarvested = "delete_plot_harvested"
)
