package converter

import (
	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/model"
	"terrion-backend/internal/plots"
	"terrion-backend/internal/usecase"
)

func PublicPlotToResponse(plot usecase.PublicPlot) *model.PublicPlotResponse {
	response := &model.PublicPlotResponse{
		PublicID:        plot.View.PublicID,
		Name:            plot.View.Name,
		AreaHa:          plot.View.AreaHa,
		TileSizeM2:      plot.View.TileSizeM2,
		MemberName:      plot.View.MemberName,
		Village:         plot.View.Village,
		District:        plot.View.District,
		TerrainSeed:     plot.View.TerrainSeed,
		Degraded:        plot.Degraded,
		CooperativeName: plot.CooperativeName,
		Blocks:          make([]model.PublicBlockResponse, len(plot.Blocks)),
		Neighbours:      neighboursToResponse(plot.Neighbours),
	}

	for i, block := range plot.Blocks {
		response.Blocks[i] = model.PublicBlockResponse{
			ID:            block.ID,
			Label:         block.Label,
			AreaHa:        block.AreaHa,
			OrderIndex:    block.OrderIndex,
			CommodityName: block.CommodityName,
			VarietyName:   block.VarietyName,
			SpriteRow:     block.SpriteRow,
			PlantingDate:  agronomy.ToISODate(block.PlantingDate),
			Window:        HarvestWindowToResponse(block.Window, true),
		}
		if block.YieldRange != nil {
			response.Blocks[i].YieldRange = &model.YieldRangeResponse{
				Min: block.YieldRange.MinTonnes,
				Max: block.YieldRange.MaxTonnes,
			}
		}
	}

	return response
}

func neighboursToResponse(neighbours plots.Neighbours) model.NeighboursResponse {
	response := model.NeighboursResponse{
		Position: neighbours.Position,
		Total:    neighbours.Total,
		Previous: neighbourToResponse(neighbours.Previous),
		Next:     neighbourToResponse(neighbours.Next),
		Others:   make([]model.PlotNeighbourResponse, len(neighbours.Others)),
	}

	for i, other := range neighbours.Others {
		response.Others[i] = *neighbourToResponse(&other)
	}
	return response
}

func neighbourToResponse(neighbour *plots.Neighbour) *model.PlotNeighbourResponse {
	if neighbour == nil {
		return nil
	}
	return &model.PlotNeighbourResponse{
		PublicID:   neighbour.PublicID,
		Name:       neighbour.Name,
		MemberName: neighbour.MemberName,
		AreaHa:     neighbour.AreaHa,
	}
}

func AtlasCooperativesToResponse(
	cooperatives []usecase.AtlasCooperative,
) []model.AtlasCooperativeResponse {
	responses := make([]model.AtlasCooperativeResponse, len(cooperatives))
	for i, cooperative := range cooperatives {
		responses[i] = model.AtlasCooperativeResponse{
			ID:        cooperative.ID,
			Name:      cooperative.Name,
			Village:   cooperative.Village,
			District:  cooperative.District,
			Province:  cooperative.Province,
			Lat:       cooperative.Lat,
			Lng:       cooperative.Lng,
			PlotCount: cooperative.PlotCount,
			Hectares:  cooperative.Hectares,
		}
	}
	return responses
}

func AtlasFarmToResponse(farm usecase.AtlasFarm) *model.AtlasFarmResponse {
	response := &model.AtlasFarmResponse{
		CooperativeID: farm.CooperativeID,
		Name:          farm.Name,
		Village:       farm.Village,
		District:      farm.District,
		Province:      farm.Province,
		Plots:         make([]model.AtlasPlotResponse, len(farm.Plots)),
		TotalHectares: farm.TotalHectares,
	}

	for i, plot := range farm.Plots {
		response.Plots[i] = model.AtlasPlotResponse{
			PublicID:   plot.PublicID,
			Name:       plot.Name,
			MemberName: plot.MemberName,
			AreaHa:     plot.AreaHa,
			Crops:      orEmptyStrings(plot.Crops),
		}
	}
	return response
}
