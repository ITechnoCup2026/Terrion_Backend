package converter

import (
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/model"
	"terrion-backend/internal/rdkk"
	"terrion-backend/internal/usecase"
)

func InputOrdersToResponse(orders []usecase.InputOrderWithLines) []model.InputOrderResponse {
	response := make([]model.InputOrderResponse, len(orders))
	for i, order := range orders {
		lines := make([]model.InputOrderLineResponse, len(order.Lines))
		for j, line := range order.Lines {
			lines[j] = model.InputOrderLineResponse{
				Item: line.Item, Quantity: line.Quantity, Unit: line.Unit,
			}
		}
		response[i] = model.InputOrderResponse{
			ID:          order.Order.ID,
			SeasonLabel: order.Order.SeasonLabel,
			Status:      string(order.Order.Status),
			CreatedAt:   order.Order.CreatedAt.UTC().Format(time.RFC3339),
			Lines:       lines,
		}
	}
	return response
}

func RdkkToResponse(document rdkk.Document, season usecase.Season) *model.RdkkResponse {
	response := &model.RdkkResponse{
		Meta: model.RdkkMetaResponse{
			CooperativeName: document.Meta.CooperativeName,
			Village:         document.Meta.Village,
			District:        document.Meta.District,
			Province:        document.Meta.Province,
			SeasonLabel:     document.Meta.SeasonLabel,
			SeasonStart:     agronomy.ToISODate(season.Start),
			SeasonEnd:       agronomy.ToISODate(season.End),
			PrintedAt:       document.Meta.PrintedAt.UTC().Format(time.RFC3339),
		},
		Columns:                 orEmptyStrings(document.Columns),
		Rows:                    make([]model.RdkkRowResponse, len(document.Rows)),
		Totals:                  orEmptyFloats(document.Totals),
		Sources:                 orEmptyStrings(document.Sources),
		MemberCount:             document.MemberCount,
		TotalPlantedHa:          document.TotalPlantedHa,
		MembersOverCap:          document.MembersOverCap,
		CommoditiesWithoutRates: orEmptyStrings(document.CommoditiesWithoutRates),
		SubsidyCapHa:            constants.SubsidyCapHa,
	}

	for i, row := range document.Rows {
		response.Rows[i] = model.RdkkRowResponse{
			MemberID:       row.MemberID,
			MemberName:     row.MemberName,
			PlantedHa:      row.PlantedHa,
			QuantitiesKg:   row.QuantitiesKg,
			OverSubsidyCap: row.OverSubsidyCap,
			ExcessHa:       row.ExcessHa,
		}
	}

	return response
}

func orEmptyStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func orEmptyFloats(values []float64) []float64 {
	if values == nil {
		return []float64{}
	}
	return values
}
