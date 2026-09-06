package converter

import (
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/planning"
	"terrion-backend/internal/rdkk"
	"terrion-backend/internal/usecase"
)

func ProposalToResponse(proposal usecase.Proposal) *model.ProposalResponse {
	response := &model.ProposalResponse{
		Season:            seasonToResponse(proposal.Season),
		Basis:             string(constants.BasisClimatology),
		Limits:            constants.PlanClimateDisclaimer,
		PreviousSeason:    previousSeasonToResponse(proposal.PreviousSeason),
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

func candidatePlanToResponse(plan usecase.ProposedPlan) model.CandidatePlanResponse {
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

		Thresholds: thresholdsToResponse(plan.Thresholds),
		Flagged:    flaggedToResponse(plan.Flagged),

		Fertiliser:        fertiliserToResponse(plan.Fertiliser.Totals),
		FertiliserUnrated: plan.Fertiliser.CommoditiesWithoutRates,
		OverSubsidyCap:    overSubsidyCapToResponse(plan.Fertiliser.Members),
	}

	if converted.FertiliserUnrated == nil {
		converted.FertiliserUnrated = []string{}
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

	response.MemberShares = memberSharesToResponse(
		stored.Items, stored.MemberNames, stored.MemberPhones, stored.ShareTokens)
	return response
}

func memberSharesToResponse(
	items []entity.SeasonPlanItem, memberNames map[string]string, memberPhones map[string]*string,
	tokens []entity.PlanShareToken,
) []model.MemberShareResponse {
	tokenByMember := make(map[string]entity.PlanShareToken, len(tokens))
	for _, token := range tokens {
		tokenByMember[token.MemberID] = token
	}

	shares := []model.MemberShareResponse{}
	seen := map[string]bool{}
	for _, item := range items {
		if seen[item.MemberID] {
			continue
		}
		seen[item.MemberID] = true

		token := tokenByMember[item.MemberID]
		share := model.MemberShareResponse{
			MemberID:    item.MemberID,
			MemberName:  memberNames[item.MemberID],
			MemberPhone: memberPhones[item.MemberID],
			ShareToken:  token.ID,
			Viewed:      token.FirstViewedAt != nil,
		}
		if token.FirstViewedAt != nil {
			firstViewed := token.FirstViewedAt.UTC().Format(time.RFC3339)
			share.FirstViewedAt = &firstViewed
		}
		shares = append(shares, share)
	}
	return shares
}

func MemberPlanShareToResponse(share usecase.MemberPlanShare) *model.MemberPlanShareResponse {
	response := &model.MemberPlanShareResponse{
		MemberName:      share.MemberName,
		CooperativeName: share.CooperativeName,
		SeasonLabel:     share.SeasonLabel,
		PlanStatus:      share.PlanStatus,
		Items:           make([]model.MemberPlanShareItemResponse, len(share.Items)),
		Fertiliser:      fertiliserToResponse(share.Fertiliser),
	}

	for i, item := range share.Items {
		response.Items[i] = model.MemberPlanShareItemResponse{
			PlotName:      item.PlotName,
			CommodityName: item.CommodityName,
			VarietyName:   item.VarietyName,
			PlantingDate:  agronomy.ToISODate(item.PlantingDate),
			HarvestStart:  agronomy.ToISODate(item.HarvestStart),
			HarvestEnd:    agronomy.ToISODate(item.HarvestEnd),
			AreaHa:        item.AreaHa,
			TonnesLow:     item.TonnesLow,
			TonnesMid:     item.TonnesMid,
			TonnesHigh:    item.TonnesHigh,
			Plausibility:  item.Plausibility,
		}
	}

	if share.OverSubsidyCap != nil {
		response.OverSubsidyCap = &model.MemberShareSubsidyCapResponse{
			PlantedHa: share.OverSubsidyCap.PlantedHa,
			ExcessHa:  share.OverSubsidyCap.ExcessHa,
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

func thresholdsToResponse(
	thresholds []agronomy.CommodityThreshold,
) []model.CommodityThresholdResponse {
	converted := make([]model.CommodityThresholdResponse, len(thresholds))
	for i, threshold := range thresholds {
		converted[i] = model.CommodityThresholdResponse{
			CommodityID:   threshold.CommodityID,
			TonnesPerWeek: threshold.TonnesPerWeek,
			Basis:         string(threshold.Basis),
		}
	}
	return converted
}

func flaggedToResponse(flagged []agronomy.FlaggedWeek) []model.PlanFlaggedWeekResponse {
	converted := make([]model.PlanFlaggedWeekResponse, len(flagged))
	for i, week := range flagged {
		converted[i] = model.PlanFlaggedWeekResponse{
			ISOWeek:         week.ISOWeek,
			CommodityID:     week.CommodityID,
			Tonnes:          week.Tonnes,
			ThresholdTonnes: week.Threshold,
			Basis:           string(week.Basis),
		}
	}
	return converted
}

func fertiliserToResponse(lines []rdkk.RequirementLine) []model.FertiliserLineResponse {
	converted := make([]model.FertiliserLineResponse, len(lines))
	for i, line := range lines {
		converted[i] = model.FertiliserLineResponse{
			InputItem:  line.InputItem,
			QuantityKg: line.QuantityKg,
			Sources:    line.Sources,
		}
	}
	return converted
}

// Hanya anggota yang benar-benar melewati batas yang muncul. Daftar kosong
// berarti tidak ada yang melewati, bukan bahwa batasnya tidak diperiksa.
func overSubsidyCapToResponse(
	members []rdkk.MemberRequirement,
) []model.OverSubsidyCapResponse {
	converted := []model.OverSubsidyCapResponse{}
	for _, member := range members {
		if !member.OverSubsidyCap {
			continue
		}
		converted = append(converted, model.OverSubsidyCapResponse{
			MemberID:   member.MemberID,
			MemberName: member.MemberName,
			PlantedHa:  member.PlantedHa,
			ExcessHa:   member.ExcessHa,
		})
	}
	return converted
}

func previousSeasonToResponse(summary *planning.SeasonSummary) *model.PreviousSeasonResponse {
	if summary == nil {
		return nil
	}
	return &model.PreviousSeasonResponse{
		Label:       summary.Label,
		PeakTonnes:  summary.PeakTonnes,
		TotalTonnes: summary.TotalTonnes,
		Blocks:      summary.Blocks,
	}
}
