package converter

import (
	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/plots"
	"terrion-backend/internal/usecase"
)

func HarvestWindowToResponse(
	window *agronomy.HarvestWindow, withSeries bool,
) *model.HarvestWindowResponse {
	if window == nil {
		return nil
	}

	response := &model.HarvestWindowResponse{
		Start:          agronomy.ToISODate(window.Start),
		End:            agronomy.ToISODate(window.End),
		Confidence:     window.Confidence,
		GddAccumulated: window.GddAccumulated,
		GddRequired:    window.GddRequired,
		Stage:          int(window.Stage),
		Basis:          string(window.Basis),
		Plausibility:   string(window.Plausibility),
	}

	if withSeries {
		response.CumulativeGdd = make([]model.CumulativeGddPoint, len(window.CumulativeGdd))
		for i, point := range window.CumulativeGdd {
			response.CumulativeGdd[i] = model.CumulativeGddPoint{Date: point.Date, Gdd: point.Gdd}
		}
		if window.ProjectedFrom != nil {
			date := agronomy.ToISODate(*window.ProjectedFrom)
			response.ProjectedFrom = &date
		}
	}

	return response
}

func PlotSummariesToResponse(summaries []plots.PlotSummary) []model.PlotSummaryResponse {
	responses := make([]model.PlotSummaryResponse, len(summaries))
	for i, summary := range summaries {
		responses[i] = model.PlotSummaryResponse{
			ID:             summary.ID,
			Name:           summary.Name,
			PublicID:       summary.PublicID,
			AreaHa:         summary.AreaHa,
			MemberName:     summary.MemberName,
			BlockCount:     summary.BlockCount,
			NextWindow:     HarvestWindowToResponse(summary.NextWindow, false),
			ExpectedTonnes: summary.ExpectedTonnes,
			CommodityIDs:   summary.CommodityIDs,
			Progress:       summary.Progress,
		}
	}
	return responses
}

func PlotDetailToResponse(detail usecase.PlotDetail) *model.PlotDetailResponse {
	response := &model.PlotDetailResponse{
		ID:                 detail.Plot.ID,
		Name:               detail.Plot.Name,
		PublicID:           detail.Plot.PublicID,
		AreaHa:             detail.Plot.AreaHa,
		TileSizeM2:         detail.Plot.TileSizeM2,
		MemberName:         detail.MemberName,
		TerrainSeed:        detail.Plot.TerrainSeed,
		Degraded:           detail.Degraded,
		HasHarvestedBlocks: detail.HasHarvestedBlocks,
		Blocks:             make([]model.PlotBlockResponse, len(detail.Blocks)),
	}

	for i, block := range detail.Blocks {
		commodity := detail.Commodities[block.CommodityID]
		variety := detail.Varieties[block.VarietyID]

		var window *agronomy.HarvestWindow
		if found, known := detail.Windows[block.ID]; known {
			window = &found
		}

		var tonnes *float64
		if expected, known := detail.Tonnes[block.ID]; known {
			tonnes = &expected
		}

		var price *model.PriceBenchmarkResponse
		if found, known := detail.Prices[block.ID]; known {
			price = priceBenchmarkToResponse(found)
		}

		response.Blocks[i] = model.PlotBlockResponse{
			ID:             block.ID,
			Label:          block.Label,
			AreaHa:         block.AreaHa,
			OrderIndex:     block.OrderIndex,
			CommodityID:    block.CommodityID,
			CommodityName:  commodity.Name,
			SpriteRow:      commodity.SpriteRow,
			VarietyID:      block.VarietyID,
			VarietyName:    variety.Name,
			PlantingDate:   agronomy.ToISODate(block.PlantingDate),
			FromPlan:       block.SeasonPlanID != nil,
			Window:         HarvestWindowToResponse(window, true),
			ExpectedTonnes: tonnes,
			Price:          price,
		}
	}

	return response
}

// Dates go out as ISO strings like every other date in the API.
func priceBenchmarkToResponse(benchmark agronomy.PriceBenchmark) *model.PriceBenchmarkResponse {
	response := &model.PriceBenchmarkResponse{
		Latest: weekPriceToResponse(benchmark.Latest),
		Source: benchmark.Source,
	}
	if benchmark.Seasonal != nil {
		seasonal := weekPriceToResponse(*benchmark.Seasonal)
		response.Seasonal = &seasonal
	}
	return response
}

func weekPriceToResponse(week agronomy.WeekPrice) model.WeekPriceResponse {
	return model.WeekPriceResponse{
		PricePerKg: week.PricePerKg,
		WeekStart:  agronomy.ToISODate(week.WeekStart),
	}
}

func CommodityCatalogueToResponse(
	commodities []entity.Commodity, varieties []entity.Variety,
) []model.CommodityResponse {
	byCommodity := map[string][]model.VarietyResponse{}
	for _, variety := range varieties {
		byCommodity[variety.CommodityID] = append(byCommodity[variety.CommodityID],
			model.VarietyResponse{
				ID:               variety.ID,
				CommodityID:      variety.CommodityID,
				Name:             variety.Name,
				DaysToHarvestMin: variety.DaysToHarvestMin,
				DaysToHarvestMax: variety.DaysToHarvestMax,
				YieldPerHaMin:    variety.YieldPerHaMin,
				YieldPerHaMax:    variety.YieldPerHaMax,
			})
	}

	responses := make([]model.CommodityResponse, len(commodities))
	for i, commodity := range commodities {
		varietyResponses := byCommodity[commodity.ID]
		if varietyResponses == nil {
			varietyResponses = []model.VarietyResponse{}
		}
		responses[i] = model.CommodityResponse{
			ID:        commodity.ID,
			Slug:      commodity.Slug,
			Name:      commodity.Name,
			SpriteRow: commodity.SpriteRow,
			Varieties: varietyResponses,
		}
	}
	return responses
}

// What the cooperative's own harvests have taught the model about one variety.
//
// AppliedOffsetDays is the number the predictor actually uses. It is the fitted
// offset shrunk toward zero by how few harvests back it, which is why a
// cooperative that has recorded two harvests sees a smaller correction than the
// raw figure beside it -- and why both are reported rather than just the one
// that sounds most impressive.
func CalibrationToResponse(
	row *entity.Calibration, varietyName, commodityName string,
) *model.CalibrationResponse {
	if row == nil {
		return nil
	}

	return &model.CalibrationResponse{
		VarietyID:     row.VarietyID,
		VarietyName:   varietyName,
		CommodityName: commodityName,
		OffsetDays:    row.OffsetDays,
		AppliedOffsetDays: agronomy.ShrunkOffset(agronomy.Calibration{
			OffsetDays:    row.OffsetDays,
			NObservations: row.NObservations,
			ResidualSd:    row.ResidualSd,
		}),
		NObservations: row.NObservations,
		ResidualSd:    row.ResidualSd,
	}
}
