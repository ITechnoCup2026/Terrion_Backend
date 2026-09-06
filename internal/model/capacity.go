package model

// CapacityRowResponse adalah satu komoditas acuan beserta kapasitas gudang
// koperasi untuknya, bila sudah ada.
//
// TonnesPerWeek null berarti belum diukur, bukan nol. Perbedaannya bekerja:
// tanpa angka, deteksi tabrakan memakai median x 2,5 sebagai ambang; dengan
// nol ia akan menandai setiap minggu yang berisi apa pun.
type CapacityRowResponse struct {
	CommodityID   string   `json:"commodity_id"`
	CommodityName string   `json:"commodity_name"`
	TonnesPerWeek *float64 `json:"tonnes_per_week"`
}

type CapacityResponse struct {
	Rows []CapacityRowResponse `json:"rows"`
}

// SetCapacityRow membawa satu komoditas. TonnesPerWeek null menghapus
// kapasitas komoditas itu, mengembalikannya ke ambang berbasis median.
type SetCapacityRow struct {
	CommodityID   string   `json:"commodity_id" validate:"required,uuid"`
	TonnesPerWeek *float64 `json:"tonnes_per_week" validate:"omitempty,gt=0"`
}

// SetCapacityRequest mengirim seluruh tabel sekaligus, bukan satu baris.
// Layarnya memang satu formulir, dan menyimpan per baris berarti separuh
// perubahan bisa tersimpan sementara separuh lainnya gagal.
type SetCapacityRequest struct {
	Rows []SetCapacityRow `json:"rows" validate:"required,dive"`
}
