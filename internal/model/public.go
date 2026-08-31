package model

type PublicBlockResponse struct {
	ID            string                 `json:"id"`
	Label         string                 `json:"label"`
	AreaHa        float64                `json:"area_ha"`
	OrderIndex    int                    `json:"order_index"`
	CommodityName string                 `json:"commodity_name"`
	VarietyName   string                 `json:"variety_name"`
	SpriteRow     int                    `json:"sprite_row"`
	PlantingDate  string                 `json:"planting_date"`
	Window        *HarvestWindowResponse `json:"window"`
	YieldRange    *YieldRangeResponse    `json:"yield_range_tonnes"`
}

type YieldRangeResponse struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type PlotNeighbourResponse struct {
	PublicID   string  `json:"public_id"`
	Name       string  `json:"name"`
	MemberName string  `json:"member_name"`
	AreaHa     float64 `json:"area_ha"`
}

type NeighboursResponse struct {
	Position int                     `json:"position"`
	Total    int                     `json:"total"`
	Previous *PlotNeighbourResponse  `json:"previous"`
	Next     *PlotNeighbourResponse  `json:"next"`
	Others   []PlotNeighbourResponse `json:"others"`
}

type PublicPlotResponse struct {
	PublicID        string                `json:"public_id"`
	Name            string                `json:"name"`
	AreaHa          float64               `json:"area_ha"`
	TileSizeM2      int                   `json:"tile_size_m2"`
	MemberName      string                `json:"member_name"`
	Village         string                `json:"village"`
	District        string                `json:"district"`
	TerrainSeed     int                   `json:"terrain_seed"`
	Degraded        bool                  `json:"degraded"`
	CooperativeName *string               `json:"cooperative_name"`
	Blocks          []PublicBlockResponse `json:"blocks"`
	Neighbours      NeighboursResponse    `json:"neighbours"`
}

type AtlasCooperativeResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Village   string  `json:"village"`
	District  string  `json:"district"`
	Province  string  `json:"province"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	PlotCount int     `json:"plot_count"`
	Hectares  float64 `json:"hectares"`
}

type AtlasPlotResponse struct {
	PublicID   string   `json:"public_id"`
	Name       string   `json:"name"`
	MemberName string   `json:"member_name"`
	AreaHa     float64  `json:"area_ha"`
	Crops      []string `json:"crops"`
}

type AtlasFarmResponse struct {
	CooperativeID string              `json:"cooperative_id"`
	Name          string              `json:"name"`
	Village       string              `json:"village"`
	District      string              `json:"district"`
	Province      string              `json:"province"`
	Plots         []AtlasPlotResponse `json:"plots"`
	TotalHectares float64             `json:"total_hectares"`
}
