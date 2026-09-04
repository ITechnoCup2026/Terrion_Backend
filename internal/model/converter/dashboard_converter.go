package converter

import (
	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/dashboard"
	"terrion-backend/internal/model"
	"terrion-backend/internal/usecase"
)

func DashboardToResponse(loaded usecase.Dashboard) *model.DashboardResponse {
	response := &model.DashboardResponse{
		Weeks:       make([]model.ProjectionWeekResponse, len(loaded.Weeks)),
		Flagged:     make([]model.FlaggedWeekResponse, len(loaded.Flagged)),
		Suggestions: make([]model.StaggerSuggestionResponse, len(loaded.Suggestions)),
		Upcoming: model.UpcomingResponse{
			Rows:        make([]model.UpcomingHarvestResponse, len(loaded.Upcoming)),
			TotalTonnes: loaded.UpcomingTonnes,
		},
		Impact: model.ImpactResponse{
			PriceVsReference: loaded.Impact.PriceVsReference,
			DaysToPayment:    loaded.Impact.DaysToPayment,
			InputCostSaved:   loaded.Impact.InputCostSaved,
			TonnesDiverted:   loaded.Impact.TonnesDiverted,
		},
		Calibrations: make([]model.CalibrationResponse, 0, len(loaded.Calibrations)),
	}

	for _, calibration := range loaded.Calibrations {
		converted := CalibrationToResponse(
			&calibration.Calibration, calibration.VarietyName, calibration.CommodityName)
		if converted != nil {
			response.Calibrations = append(response.Calibrations, *converted)
		}
	}

	for i, week := range loaded.Weeks {
		response.Weeks[i] = model.ProjectionWeekResponse{
			ISOWeek:        week.ISOWeek,
			WeekStart:      agronomy.ToISODate(week.WeekStart),
			ExpectedTonnes: week.ExpectedTonnes,
			MinTonnes:      week.MinTonnes,
			MaxTonnes:      week.MaxTonnes,
			BlockIDs:       week.BlockIDs,
		}
	}

	for i, week := range loaded.Flagged {
		response.Flagged[i] = flaggedWeekToResponse(week, loaded.Commodities)
	}

	if loaded.Lead != nil {
		lead := flaggedWeekToResponse(*loaded.Lead, loaded.Commodities)
		response.Lead = &lead
	}

	for i, suggestion := range loaded.Suggestions {
		response.Suggestions[i] = model.StaggerSuggestionResponse{
			ISOWeek:         suggestion.ISOWeek,
			CommodityID:     suggestion.CommodityID,
			CommodityName:   commodityName(loaded.Commodities, suggestion.CommodityID),
			BlockIDs:        suggestion.BlockIDs,
			ShiftDays:       suggestion.ShiftDays,
			TonnesMoved:     suggestion.TonnesMoved,
			ResultingTonnes: suggestion.ResultingTonnes,
		}
	}

	for i, row := range loaded.Upcoming {
		response.Upcoming.Rows[i] = model.UpcomingHarvestResponse{
			BlockID:       row.BlockID,
			PlotID:        row.PlotID,
			PlotName:      row.PlotName,
			MemberName:    row.MemberName,
			CommodityName: row.CommodityName,
			Tonnes:        row.Tonnes,
			Start:         agronomy.ToISODate(row.Start),
			End:           agronomy.ToISODate(row.End),
		}
	}

	return response
}

func flaggedWeekToResponse(
	week dashboard.CollisionWeek, commodities map[string]string,
) model.FlaggedWeekResponse {
	return model.FlaggedWeekResponse{
		ISOWeek:       week.ISOWeek,
		WeekStart:     agronomy.ToISODate(week.WeekStart),
		CommodityID:   week.CommodityID,
		CommodityName: commodityName(commodities, week.CommodityID),
		Tonnes:        week.Tonnes,
		Threshold:     week.Threshold,
		Basis:         string(week.Basis),
		PlotCount:     week.PlotCount,
		BlockIDs:      week.ContributingBlockIDs,
	}
}

func commodityName(commodities map[string]string, commodityID string) string {
	if name, known := commodities[commodityID]; known {
		return name
	}
	return "Komoditas"
}
