package converter

import (
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/catalog"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/usecase"
)

func ListingsToResponse(listings []catalog.Listing) []model.ListingResponse {
	responses := make([]model.ListingResponse, len(listings))
	for i, listing := range listings {
		responses[i] = model.ListingResponse{
			ID:              listing.ID,
			CooperativeID:   listing.CooperativeID,
			CooperativeName: listing.CooperativeName,
			Province:        listing.Province,
			District:        listing.District,
			Village:         listing.Village,
			CommodityID:     listing.CommodityID,
			CommodityName:   listing.CommodityName,
			VarietyName:     listing.VarietyName,
			ISOWeek:         listing.ISOWeek,
			WeekStart:       agronomy.ToISODate(listing.WeekStart),
			WeekEnd:         agronomy.ToISODate(listing.WeekEnd),
			Tonnes:          listing.Tonnes,
			Basis:           string(listing.Basis),
		}
	}
	return responses
}

func CatalogToResponse(built usecase.Catalog) *model.CatalogResponse {
	commodities := make([]model.CatalogFilterOption, len(built.Commodities))
	for i, commodity := range built.Commodities {
		commodities[i] = model.CatalogFilterOption{ID: commodity.ID, Name: commodity.Name}
	}

	return &model.CatalogResponse{
		Listings:    ListingsToResponse(built.Listings),
		Commodities: commodities,
		Provinces:   orEmptyStrings(built.Provinces),
	}
}

func SupplyRequestToResponse(request *entity.SupplyContractRequest) *model.SupplyRequestResponse {
	response := &model.SupplyRequestResponse{
		ID:                request.ID,
		CooperativeID:     request.CooperativeID,
		BuyerID:           request.BuyerID,
		BuyerName:         request.BuyerName,
		BuyerOrganisation: request.BuyerOrganisation,
		CommodityID:       request.CommodityID,
		VolumeKg:          request.VolumeKg,
		WindowStart:       agronomy.ToISODate(request.WindowStart),
		WindowEnd:         agronomy.ToISODate(request.WindowEnd),
		Status:            string(request.Status),
		Notes:             request.Notes,
		CreatedAt:         request.CreatedAt.UTC().Format(time.RFC3339),
	}

	if request.RespondedAt != nil {
		respondedAt := request.RespondedAt.UTC().Format(time.RFC3339)
		response.RespondedAt = &respondedAt
	}

	return response
}

func SupplyRequestsToResponse(
	requests []entity.SupplyContractRequest,
) []model.SupplyRequestResponse {
	responses := make([]model.SupplyRequestResponse, len(requests))
	for i := range requests {
		responses[i] = *SupplyRequestToResponse(&requests[i])
	}
	return responses
}
