package model

import "terrion-backend/internal/constants"

type ListingResponse struct {
	ID              string  `json:"id"`
	CooperativeID   string  `json:"cooperative_id"`
	CooperativeName string  `json:"cooperative_name"`
	Province        string  `json:"province"`
	District        string  `json:"district"`
	Village         string  `json:"village"`
	CommodityID     string  `json:"commodity_id"`
	CommodityName   string  `json:"commodity_name"`
	VarietyName     *string `json:"variety_name"`
	ISOWeek         string  `json:"iso_week"`
	WeekStart       string  `json:"week_start"`
	WeekEnd         string  `json:"week_end"`
	Tonnes          float64 `json:"tonnes"`
	Basis           string  `json:"basis"`
}

type CatalogFilterOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CatalogResponse struct {
	Listings    []ListingResponse     `json:"listings"`
	Commodities []CatalogFilterOption `json:"commodities"`
	Provinces   []string              `json:"provinces"`
}

type CreateSupplyRequestRequest struct {
	ListingID          string                       `json:"listing_id" validate:"required"`
	VolumeTonnes       float64                      `json:"volume_tonnes" validate:"required,gt=0,max=100000"`
	DeliveryPreference constants.DeliveryPreference `json:"delivery_preference" validate:"required,oneof=antar_ke_gudang ambil_di_koperasi belum_ditentukan"`
	Notes              string                       `json:"notes" validate:"max=1000"`
}

type RespondToRequestRequest struct {
	Decision constants.RequestStatus `json:"decision" validate:"required,oneof=accepted declined"`
}

type SupplyRequestResponse struct {
	ID                string  `json:"id"`
	CooperativeID     string  `json:"cooperative_id"`
	BuyerID           string  `json:"buyer_id"`
	BuyerName         string  `json:"buyer_name"`
	BuyerOrganisation *string `json:"buyer_organisation"`
	CommodityID       string  `json:"commodity_id"`
	VolumeKg          float64 `json:"volume_kg"`
	WindowStart       string  `json:"window_start"`
	WindowEnd         string  `json:"window_end"`
	Status            string  `json:"status"`
	Notes             *string `json:"notes"`
	CreatedAt         string  `json:"created_at"`
	RespondedAt       *string `json:"responded_at"`
}
