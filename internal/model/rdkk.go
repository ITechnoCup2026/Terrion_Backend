package model

type RdkkMetaResponse struct {
	CooperativeName string `json:"cooperative_name"`
	Village         string `json:"village"`
	District        string `json:"district"`
	Province        string `json:"province"`
	SeasonLabel     string `json:"season_label"`
	SeasonStart     string `json:"season_start"`
	SeasonEnd       string `json:"season_end"`
	PrintedAt       string `json:"printed_at"`
}

type RdkkRowResponse struct {
	MemberID       string     `json:"member_id"`
	MemberName     string     `json:"member_name"`
	PlantedHa      float64    `json:"planted_ha"`
	QuantitiesKg   []*float64 `json:"quantities_kg"`
	OverSubsidyCap bool       `json:"over_subsidy_cap"`
	ExcessHa       float64    `json:"excess_ha"`
}

type RdkkResponse struct {
	Meta                    RdkkMetaResponse  `json:"meta"`
	Columns                 []string          `json:"columns"`
	Rows                    []RdkkRowResponse `json:"rows"`
	Totals                  []float64         `json:"totals"`
	Sources                 []string          `json:"sources"`
	MemberCount             int               `json:"member_count"`
	TotalPlantedHa          float64           `json:"total_planted_ha"`
	MembersOverCap          int               `json:"members_over_cap"`
	CommoditiesWithoutRates []string          `json:"commodities_without_rates"`
	SubsidyCapHa            float64           `json:"subsidy_cap_ha"`
}

type CreateInputOrderResponse struct {
	OrderID string `json:"order_id"`
	Lines   int    `json:"lines"`
}
