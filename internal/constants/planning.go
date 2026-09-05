package constants

type PlanningObjective string

const (
	ObjectiveSafe   PlanningObjective = "aman"
	ObjectiveIncome PlanningObjective = "pendapatan"
	ObjectiveMarket PlanningObjective = "pasar"
)

type PlanStatus string

const (
	PlanApplied   PlanStatus = "applied"
	PlanCancelled PlanStatus = "cancelled"
)

const (
	PlanNoPlots              = "plan_no_plots"
	PlanNoClimateNormals     = "plan_no_climate_normals"
	PlanSeasonClosed         = "plan_season_closed"
	PlanNoEligiblePlots      = "plan_no_eligible_plots"
	PlanAlreadyApplied       = "plan_already_applied"
	PlanAlreadyCancelled     = "plan_already_cancelled"
	PlanNotFound             = "plan_not_found"
	PlanAssignmentRejected   = "plan_assignment_rejected"
	PlanPartiallyCancellable = "plan_partially_cancellable"
)

const (
	PlanningLocalSearchPasses = 3
	PlanningEvaluationBudget  = 200000
)
