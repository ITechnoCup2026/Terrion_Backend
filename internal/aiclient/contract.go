package aiclient

const (
	ContractVersion = "1.0"
	ContractMajor   = "1"

	MaxCandidates = 2000
	MaxDemandRows = 400
)

type Season struct {
	Label string `json:"label"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type Candidate struct {
	ID           string   `json:"id"`
	PlotRef      string   `json:"plot_ref"`
	AreaHa       float64  `json:"area_ha"`
	CommodityRef string   `json:"commodity_ref"`
	VarietyRef   string   `json:"variety_ref"`
	PlantingDate string   `json:"planting_date"`
	HarvestStart string   `json:"harvest_start"`
	HarvestEnd   string   `json:"harvest_end"`
	TonnesLow    float64  `json:"tonnes_low"`
	TonnesMid    float64  `json:"tonnes_mid"`
	TonnesHigh   float64  `json:"tonnes_high"`
	Plausibility string   `json:"plausibility"`
	PricePerKg   *float64 `json:"price_per_kg"`
}

type DemandRow struct {
	CommodityRef string `json:"commodity_ref"`
	ISOWeek      string `json:"iso_week"`
	Kg           int    `json:"kg"`
}

type Observation struct {
	GddRatio   float64 `json:"gdd_ratio"`
	AreaHa     float64 `json:"area_ha"`
	MeanTempC  float64 `json:"mean_temp_c"`
	YieldIndex float64 `json:"yield_index"`
}

type Request struct {
	ContractVersion       string        `json:"contract_version"`
	RequestID             string        `json:"request_id"`
	Seed                  int64         `json:"seed"`
	Season                Season        `json:"season"`
	Objectives            []string      `json:"objectives"`
	CapacityTonnesPerWeek *float64      `json:"capacity_tonnes_per_week"`
	Candidates            []Candidate   `json:"candidates"`
	Demand                []DemandRow   `json:"demand"`
	Observations          []Observation `json:"observations,omitempty"`
}

type Metrics struct {
	PeakTonnesP50   float64  `json:"peak_tonnes_p50"`
	PeakTonnesP90   float64  `json:"peak_tonnes_p90"`
	TotalTonnes     float64  `json:"total_tonnes"`
	GrossValue      *float64 `json:"gross_value"`
	DemandCoveredKg int      `json:"demand_covered_kg"`
}

type PlanResult struct {
	Objective       string   `json:"objective"`
	CandidateIDs    []string `json:"candidate_ids"`
	Metrics         Metrics  `json:"metrics"`
	Narrative       *string  `json:"narrative"`
	NarrativeSource string   `json:"narrative_source"`
}

type Diagnostics struct {
	Evaluations     int      `json:"evaluations"`
	MonteCarloDraws int      `json:"monte_carlo_draws"`
	ObjectiveStatus string   `json:"objective_status"`
	Degraded        []string `json:"degraded"`
}

type Response struct {
	ContractVersion string       `json:"contract_version"`
	RequestID       string       `json:"request_id"`
	Solver          string       `json:"solver"`
	SolverVersion   string       `json:"solver_version"`
	ElapsedMs       int          `json:"elapsed_ms"`
	Plans           []PlanResult `json:"plans"`
	Diagnostics     Diagnostics  `json:"diagnostics"`
}

type ErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
