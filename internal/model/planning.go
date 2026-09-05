package model

type ProposePlanRequest struct {
	SeasonLabel string   `json:"season_label" validate:"required,min=1,max=60"`
	SeasonStart string   `json:"season_start" validate:"required,datetime=2006-01-02"`
	SeasonEnd   string   `json:"season_end" validate:"required,datetime=2006-01-02"`
	Objectives  []string `json:"objectives" validate:"omitempty,max=3,dive,oneof=aman pendapatan pasar"`
}

type SeasonResponse struct {
	Label string `json:"label"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// One planting the plan proposes: this plot, this variety, this date.
//
// Carries the real domain identifiers rather than the opaque candidate
// reference, because applying a plan must not depend on a candidate list
// staying byte-identical between two requests.
type PlanAssignmentResponse struct {
	CandidateID  string  `json:"candidate_id"`
	PlotID       string  `json:"plot_id"`
	PlotName     string  `json:"plot_name"`
	CommodityID  string  `json:"commodity_id"`
	VarietyID    string  `json:"variety_id"`
	AreaHa       float64 `json:"area_ha"`
	PlantingDate string  `json:"planting_date"`
	HarvestStart string  `json:"harvest_start"`
	HarvestEnd   string  `json:"harvest_end"`
	TonnesLow    float64 `json:"tonnes_low"`
	TonnesMid    float64 `json:"tonnes_mid"`
	TonnesHigh   float64 `json:"tonnes_high"`
	Plausibility string  `json:"plausibility"`
}

// Every number here is recomputed by this service from its own candidate
// table. Nothing numeric is ever carried over from the AI service.
//
// PeakTonnesP50 and P90 are nil when the plan came from the Go solver: those
// quantiles need the Monte Carlo that only the AI service runs, and reporting
// a substitute under the same name would be a lie about how it was obtained.
//
// GrossValueSource carries the provenance of the reference price panel all
// the way to the screen. While it reads SINTETIS, the rupiah figure is a
// relative ranking between plans and must not be presented as budgetable.
type PlanMetricsResponse struct {
	ExpectedPeakTonnes    float64  `json:"expected_peak_tonnes"`
	WorstCasePeakTonnes   float64  `json:"worst_case_peak_tonnes"`
	PeakTonnesP50         *float64 `json:"peak_tonnes_p50"`
	PeakTonnesP90         *float64 `json:"peak_tonnes_p90"`
	TotalTonnes           float64  `json:"total_tonnes"`
	GrossValue            *float64 `json:"gross_value"`
	GrossValueSource      *string  `json:"gross_value_source"`
	DemandCoveredKg       int      `json:"demand_covered_kg"`
	CapacityTonnesPerWeek *float64 `json:"capacity_tonnes_per_week"`
}

type PlanResponse struct {
	Objective       string                   `json:"objective"`
	Assignments     []PlanAssignmentResponse `json:"assignments"`
	Metrics         PlanMetricsResponse      `json:"metrics"`
	Narrative       *string                  `json:"narrative"`
	NarrativeSource string                   `json:"narrative_source"`
}

type PlanDiagnosticsResponse struct {
	CandidateCount int      `json:"candidate_count"`
	Evaluations    int      `json:"evaluations"`
	Degraded       []string `json:"degraded"`
}

// Engine says which machine produced the assignments: "ai-service" or
// "fallback". It is reported rather than hidden so nobody has to guess which
// numbers came from where.
type ProposePlanResponse struct {
	Season      SeasonResponse          `json:"season"`
	Engine      string                  `json:"engine"`
	Plans       []PlanResponse          `json:"plans"`
	Diagnostics PlanDiagnosticsResponse `json:"diagnostics"`
}

type ApplyPlanAssignment struct {
	PlotID       string  `json:"plot_id" validate:"required,uuid"`
	CommodityID  string  `json:"commodity_id" validate:"required,uuid"`
	VarietyID    string  `json:"variety_id" validate:"required,uuid"`
	AreaHa       float64 `json:"area_ha" validate:"required,gt=0"`
	PlantingDate string  `json:"planting_date" validate:"required,datetime=2006-01-02"`
	Label        string  `json:"label" validate:"omitempty,max=60"`
}

type ApplyPlanRequest struct {
	SeasonLabel string                `json:"season_label" validate:"required,min=1,max=60"`
	SeasonStart string                `json:"season_start" validate:"required,datetime=2006-01-02"`
	SeasonEnd   string                `json:"season_end" validate:"required,datetime=2006-01-02"`
	Objective   string                `json:"objective" validate:"required,oneof=aman pendapatan pasar"`
	Engine      string                `json:"engine" validate:"required,oneof=ai-service fallback"`
	Assignments []ApplyPlanAssignment `json:"assignments" validate:"required,min=1,max=500,dive"`
}

type ApplyPlanResponse struct {
	PlanID           string `json:"plan_id"`
	BlocksCreated    int    `json:"blocks_created"`
	Objective        string `json:"objective"`
	SeasonLabel      string `json:"season_label"`
	SeasonStart      string `json:"season_start"`
	ReplacedExisting bool   `json:"replaced_existing"`
}
