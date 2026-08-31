package model

type ProjectionWeekResponse struct {
	ISOWeek        string   `json:"iso_week"`
	WeekStart      string   `json:"week_start"`
	ExpectedTonnes float64  `json:"expected_tonnes"`
	MinTonnes      float64  `json:"min_tonnes"`
	MaxTonnes      float64  `json:"max_tonnes"`
	BlockIDs       []string `json:"block_ids"`
}

type FlaggedWeekResponse struct {
	ISOWeek       string   `json:"iso_week"`
	WeekStart     string   `json:"week_start"`
	CommodityID   string   `json:"commodity_id"`
	CommodityName string   `json:"commodity_name"`
	Tonnes        float64  `json:"tonnes"`
	Threshold     float64  `json:"threshold"`
	Basis         string   `json:"basis"`
	PlotCount     int      `json:"plot_count"`
	BlockIDs      []string `json:"block_ids"`
}

type StaggerSuggestionResponse struct {
	ISOWeek         string   `json:"iso_week"`
	CommodityID     string   `json:"commodity_id"`
	CommodityName   string   `json:"commodity_name"`
	BlockIDs        []string `json:"block_ids"`
	ShiftDays       int      `json:"shift_days"`
	TonnesMoved     float64  `json:"tonnes_moved"`
	ResultingTonnes float64  `json:"resulting_tonnes"`
}

type UpcomingHarvestResponse struct {
	BlockID       string  `json:"block_id"`
	PlotID        string  `json:"plot_id"`
	PlotName      string  `json:"plot_name"`
	MemberName    *string `json:"member_name"`
	CommodityName string  `json:"commodity_name"`
	Tonnes        float64 `json:"tonnes"`
	Start         string  `json:"start"`
	End           string  `json:"end"`
}

type UpcomingResponse struct {
	Rows        []UpcomingHarvestResponse `json:"rows"`
	TotalTonnes float64                   `json:"total_tonnes"`
}

type ImpactResponse struct {
	PriceVsReference *float64 `json:"price_vs_reference"`
	DaysToPayment    *float64 `json:"days_to_payment"`
	InputCostSaved   *float64 `json:"input_cost_saved"`
	TonnesDiverted   *float64 `json:"tonnes_diverted"`
}

type DashboardResponse struct {
	Weeks       []ProjectionWeekResponse    `json:"weeks"`
	Flagged     []FlaggedWeekResponse       `json:"flagged"`
	Lead        *FlaggedWeekResponse        `json:"lead"`
	Suggestions []StaggerSuggestionResponse `json:"suggestions"`
	Upcoming    UpcomingResponse            `json:"upcoming"`
	Impact      ImpactResponse              `json:"impact"`
}

type ApplyStaggerRequest struct {
	ISOWeek     string `json:"iso_week" validate:"required"`
	CommodityID string `json:"commodity_id" validate:"required,uuid"`
}

type ApplyStaggerResponse struct {
	Shifted int `json:"shifted"`
}

type StaggerRefusalResponse struct {
	AlreadyPlanted int `json:"already_planted"`
	WouldBeInPast  int `json:"would_be_in_the_past"`
}
