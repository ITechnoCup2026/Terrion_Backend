package model

type SeasonPlanAssignmentRequest struct {
	PlotID       string `json:"plot_id" validate:"required"`
	VarietyID    string `json:"variety_id" validate:"required"`
	PlantingDate string `json:"planting_date" validate:"required,datetime=2006-01-02"`
}

type ApplySeasonPlanRequest struct {
	SeasonLabel string                        `json:"season_label" validate:"required,min=3"`
	Objective   string                        `json:"objective" validate:"required,oneof=aman pendapatan pasar"`
	Assignments []SeasonPlanAssignmentRequest `json:"assignments" validate:"required,min=1,dive"`
}

type SeasonResponse struct {
	Label        string `json:"label"`
	Start        string `json:"start"`
	End          string `json:"end"`
	PlantingFrom string `json:"planting_from"`
	PlantingTo   string `json:"planting_to"`
}

type PlanAssignmentResponse struct {
	PlotID       string  `json:"plot_id"`
	PlotName     string  `json:"plot_name"`
	MemberID     string  `json:"member_id"`
	MemberName   string  `json:"member_name"`
	AreaHa       float64 `json:"area_ha"`
	CommodityID  string  `json:"commodity_id"`
	VarietyID    string  `json:"variety_id"`
	VarietyName  string  `json:"variety_name"`
	PlantingDate string  `json:"planting_date"`
	HarvestStart string  `json:"harvest_start"`
	HarvestEnd   string  `json:"harvest_end"`
	Plausibility string  `json:"plausibility"`
	TonnesLow    float64 `json:"tonnes_low"`
	TonnesMid    float64 `json:"tonnes_mid"`
	TonnesHigh   float64 `json:"tonnes_high"`
}

type PlanMetricsResponse struct {
	PeakTonnesExpected float64  `json:"peak_tonnes_expected"`
	PeakTonnesWorst    float64  `json:"peak_tonnes_worst"`
	GrossValue         *float64 `json:"gross_value"`
	DemandCoveredKg    float64  `json:"demand_covered_kg"`
	TotalTonnesMid     float64  `json:"total_tonnes_mid"`
	FlaggedWeeks       int      `json:"flagged_weeks"`
}

type CandidatePlanResponse struct {
	Objective   string                   `json:"objective"`
	Narrative   string                   `json:"narrative"`
	Metrics     PlanMetricsResponse      `json:"metrics"`
	Assignments []PlanAssignmentResponse `json:"assignments"`
}

type SkippedPlotResponse struct {
	PlotID     string `json:"plot_id"`
	PlotName   string `json:"plot_name"`
	MemberName string `json:"member_name"`
	Reason     string `json:"reason"`
}

type ProposalResponse struct {
	Season            SeasonResponse          `json:"season"`
	Basis             string                  `json:"basis"`
	Engine            string                  `json:"engine"`
	YieldObservations int                     `json:"yield_observations"`
	Plans             []CandidatePlanResponse `json:"plans"`
	Skipped           []SkippedPlotResponse   `json:"skipped"`
	Evaluations       int                     `json:"evaluations"`
}

type SeasonPlanItemResponse struct {
	ID            string  `json:"id"`
	PlotID        string  `json:"plot_id"`
	PlotName      string  `json:"plot_name"`
	MemberID      string  `json:"member_id"`
	MemberName    string  `json:"member_name"`
	CommodityID   string  `json:"commodity_id"`
	CommodityName string  `json:"commodity_name"`
	VarietyID     string  `json:"variety_id"`
	VarietyName   string  `json:"variety_name"`
	AreaHa        float64 `json:"area_ha"`
	PlantingDate  string  `json:"planting_date"`
	HarvestStart  string  `json:"harvest_start"`
	HarvestEnd    string  `json:"harvest_end"`
	Plausibility  string  `json:"plausibility"`
	TonnesLow     float64 `json:"tonnes_low"`
	TonnesMid     float64 `json:"tonnes_mid"`
	TonnesHigh    float64 `json:"tonnes_high"`
	BlockID       *string `json:"block_id"`
}

type SeasonPlanResponse struct {
	ID          string                   `json:"id"`
	SeasonLabel string                   `json:"season_label"`
	SeasonStart string                   `json:"season_start"`
	SeasonEnd   string                   `json:"season_end"`
	Objective   string                   `json:"objective"`
	Status      string                   `json:"status"`
	CreatedAt   string                   `json:"created_at"`
	CancelledAt *string                  `json:"cancelled_at"`
	Items       []SeasonPlanItemResponse `json:"items"`
}

type SeasonPlanListResponse struct {
	Plans []SeasonPlanResponse `json:"plans"`
}

type ApplySeasonPlanResponse struct {
	PlanID string `json:"plan_id"`
	Blocks int    `json:"blocks"`
}

type CancelSeasonPlanResponse struct {
	PlanID        string `json:"plan_id"`
	BlocksRemoved int    `json:"blocks_removed"`
}
