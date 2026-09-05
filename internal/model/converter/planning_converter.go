package converter

import (
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/planning"
	"terrion-backend/internal/usecase"
)

func ProposalToResponse(proposal usecase.Proposal) *model.ProposalResponse {
	response := &model.ProposalResponse{
		Season:            seasonToResponse(proposal.Season),
		Basis:             string(constants.BasisClimatology),
		Engine:            string(proposal.Engine),
		YieldObservations: proposal.YieldObservations,
		Plans:             make([]model.CandidatePlanResponse, len(proposal.Plans)),
		Skipped:           make([]model.SkippedPlotResponse, len(proposal.Skipped)),
	}

	for i, plan := range proposal.Plans {
		response.Plans[i] = candidatePlanToResponse(plan)
		response.Evaluations = plan.Evaluations
	}

	for i, skipped := range proposal.Skipped {
		response.Skipped[i] = model.SkippedPlotResponse{
			PlotID:     skipped.PlotID,
			PlotName:   skipped.PlotName,
			MemberName: skipped.MemberName,
			Reason:     skipped.Reason,
		}
	}
	return response
}

func seasonToResponse(season planning.Season) model.SeasonResponse {
	return model.SeasonResponse{
		Label:        season.Label,
		Start:        agronomy.ToISODate(season.Start),
		End:          agronomy.ToISODate(season.End),
		PlantingFrom: agronomy.ToISODate(season.PlantingFrom),
		PlantingTo:   agronomy.ToISODate(season.PlantingTo),
	}
}

func candidatePlanToResponse(plan planning.Plan) model.CandidatePlanResponse {
	converted := model.CandidatePlanResponse{
		Objective: string(plan.Objective),
		Narrative: plan.Narrative,
		Metrics: model.PlanMetricsResponse{
			PeakTonnesExpected: plan.Metrics.PeakTonnesExpected,
			PeakTonnesWorst:    plan.Metrics.PeakTonnesWorst,
			GrossValue:         plan.Metrics.GrossValue,
			DemandCoveredKg:    plan.Metrics.DemandCoveredKg,
			TotalTonnesMid:     plan.Metrics.TotalTonnesMid,
			FlaggedWeeks:       len(plan.Flagged),
		},
		Assignments: make([]model.PlanAssignmentResponse, len(plan.Assignments)),
	}

	for i, assignment := range plan.Assignments {
		converted.Assignments[i] = model.PlanAssignmentResponse{
			PlotID:       assignment.PlotID,
			PlotName:     assignment.PlotName,
			MemberID:     assignment.MemberID,
			MemberName:   assignment.MemberName,
			AreaHa:       assignment.AreaHa,
			CommodityID:  assignment.CommodityID,
			VarietyID:    assignment.VarietyID,
			VarietyName:  assignment.VarietyName,
			PlantingDate: agronomy.ToISODate(assignment.PlantingDate),
			HarvestStart: agronomy.ToISODate(assignment.Window.Start),
			HarvestEnd:   agronomy.ToISODate(assignment.Window.End),
			Plausibility: string(assignment.Plausibility),
			TonnesLow:    assignment.TonnesLow,
			TonnesMid:    assignment.TonnesMid,
			TonnesHigh:   assignment.TonnesHigh,
		}
	}
	return converted
}

func StoredPlanToResponse(stored usecase.StoredPlan) *model.SeasonPlanResponse {
	response := planToResponse(stored.Plan)
	response.Items = make([]model.SeasonPlanItemResponse, len(stored.Items))

	for i, item := range stored.Items {
		response.Items[i] = model.SeasonPlanItemResponse{
			ID:            item.ID,
			PlotID:        item.PlotID,
			PlotName:      stored.PlotNames[item.PlotID],
			MemberID:      item.MemberID,
			MemberName:    stored.MemberNames[item.MemberID],
			CommodityID:   item.CommodityID,
			CommodityName: stored.CommodityNames[item.CommodityID],
			VarietyID:     item.VarietyID,
			VarietyName:   stored.VarietyNames[item.VarietyID],
			AreaHa:        item.AreaHa,
			PlantingDate:  agronomy.ToISODate(item.PlantingDate),
			HarvestStart:  agronomy.ToISODate(item.ExpectedHarvestStart),
			HarvestEnd:    agronomy.ToISODate(item.ExpectedHarvestEnd),
			Plausibility:  item.Plausibility,
			TonnesLow:     item.ExpectedTonnesLow,
			TonnesMid:     item.ExpectedTonnesMid,
			TonnesHigh:    item.ExpectedTonnesHigh,
			BlockID:       item.BlockID,
		}
	}
	return response
}

func SeasonPlansToResponse(plans []entity.SeasonPlan) *model.SeasonPlanListResponse {
	response := &model.SeasonPlanListResponse{
		Plans: make([]model.SeasonPlanResponse, len(plans)),
	}
	for i, plan := range plans {
		response.Plans[i] = *planToResponse(plan)
	}
	return response
}

func planToResponse(plan entity.SeasonPlan) *model.SeasonPlanResponse {
	response := &model.SeasonPlanResponse{
		ID:          plan.ID,
		SeasonLabel: plan.SeasonLabel,
		SeasonStart: agronomy.ToISODate(plan.SeasonStart),
		SeasonEnd:   agronomy.ToISODate(plan.SeasonEnd),
		Objective:   string(plan.Objective),
		Status:      string(plan.Status),
		CreatedAt:   plan.CreatedAt.UTC().Format(time.RFC3339),
		Items:       []model.SeasonPlanItemResponse{},
	}
	if plan.CancelledAt != nil {
		cancelled := plan.CancelledAt.UTC().Format(time.RFC3339)
		response.CancelledAt = &cancelled
	}
	return response
}
