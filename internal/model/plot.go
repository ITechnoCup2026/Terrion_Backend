package model

type PlantingRequest struct {
	CommodityID  string  `json:"commodity_id" validate:"required,uuid"`
	VarietyID    string  `json:"variety_id" validate:"required,uuid"`
	PlantingDate string  `json:"planting_date" validate:"required,datetime=2006-01-02"`
	AreaHa       float64 `json:"area_ha" validate:"required,min=0.01,max=1000"`
}

type CreatePlotRequest struct {
	MemberName string            `json:"member_name" validate:"required,min=2"`
	PlotName   string            `json:"plot_name" validate:"required,min=1"`
	Lat        *float64          `json:"lat" validate:"required,min=-11,max=6"`
	Lng        *float64          `json:"lng" validate:"required,min=95,max=141"`
	Plantings  []PlantingRequest `json:"plantings" validate:"required,min=1,max=6,dive"`
}

type CreatePlotResponse struct {
	PlotID   string `json:"plot_id"`
	PublicID string `json:"public_id"`
}

type SplitBlockRequest struct {
	AreaHa       float64 `json:"area_ha" validate:"required,min=0.01,max=1000"`
	CommodityID  string  `json:"commodity_id" validate:"required,uuid"`
	VarietyID    string  `json:"variety_id" validate:"required,uuid"`
	PlantingDate string  `json:"planting_date" validate:"required,datetime=2006-01-02"`
}

type SplitBlockResponse struct {
	PlotID  string `json:"plot_id"`
	BlockID string `json:"block_id"`
}

type SplitRefusalResponse struct {
	MinHa         float64 `json:"min_ha,omitempty"`
	BlockAreaHa   float64 `json:"block_area_ha,omitempty"`
	MaxTakeableHa float64 `json:"max_takeable_ha,omitempty"`
}

type CumulativeGddPoint struct {
	Date string  `json:"date"`
	Gdd  float64 `json:"gdd"`
}

type HarvestWindowResponse struct {
	Start          string               `json:"start"`
	End            string               `json:"end"`
	Confidence     float64              `json:"confidence"`
	GddAccumulated float64              `json:"gdd_accumulated"`
	GddRequired    float64              `json:"gdd_required"`
	Stage          int                  `json:"stage"`
	Basis          string               `json:"basis"`
	Plausibility   string               `json:"plausibility"`
	CumulativeGdd  []CumulativeGddPoint `json:"cumulative_gdd,omitempty"`
}

type PlotSummaryResponse struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	PublicID       string                 `json:"public_id"`
	AreaHa         float64                `json:"area_ha"`
	MemberName     *string                `json:"member_name"`
	BlockCount     int                    `json:"block_count"`
	NextWindow     *HarvestWindowResponse `json:"next_window"`
	ExpectedTonnes *float64               `json:"expected_tonnes"`
	CommodityIDs   []string               `json:"commodity_ids"`
	Progress       *float64               `json:"progress"`
}

type PlotBlockResponse struct {
	ID             string                 `json:"id"`
	Label          string                 `json:"label"`
	AreaHa         float64                `json:"area_ha"`
	OrderIndex     int                    `json:"order_index"`
	CommodityID    string                 `json:"commodity_id"`
	CommodityName  string                 `json:"commodity_name"`
	SpriteRow      int                    `json:"sprite_row"`
	VarietyID      string                 `json:"variety_id"`
	VarietyName    string                 `json:"variety_name"`
	PlantingDate   string                 `json:"planting_date"`
	Window         *HarvestWindowResponse `json:"window"`
	ExpectedTonnes *float64               `json:"expected_tonnes"`
}

type PlotDetailResponse struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	PublicID           string              `json:"public_id"`
	AreaHa             float64             `json:"area_ha"`
	TileSizeM2         int                 `json:"tile_size_m2"`
	MemberName         string              `json:"member_name"`
	TerrainSeed        int                 `json:"terrain_seed"`
	Degraded           bool                `json:"degraded"`
	HasHarvestedBlocks bool                `json:"has_harvested_blocks"`
	Blocks             []PlotBlockResponse `json:"blocks"`
}

type VarietyResponse struct {
	ID               string  `json:"id"`
	CommodityID      string  `json:"commodity_id"`
	Name             string  `json:"name"`
	DaysToHarvestMin int     `json:"days_to_harvest_min"`
	DaysToHarvestMax int     `json:"days_to_harvest_max"`
	YieldPerHaMin    float64 `json:"yield_per_ha_min"`
	YieldPerHaMax    float64 `json:"yield_per_ha_max"`
}

type CommodityResponse struct {
	ID        string            `json:"id"`
	Slug      string            `json:"slug"`
	Name      string            `json:"name"`
	SpriteRow int               `json:"sprite_row"`
	Varieties []VarietyResponse `json:"varieties"`
}
