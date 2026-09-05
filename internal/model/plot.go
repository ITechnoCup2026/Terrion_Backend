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

// What a kader types when a block comes in. The price and the payment date are
// optional because they are frequently not known on the day: the crop leaves
// the field before the buyer settles, and forcing a number there would mean
// inventing one.
type RecordHarvestRequest struct {
	ActualHarvestDate   string   `json:"actual_harvest_date" validate:"required,datetime=2006-01-02"`
	ActualYieldKg       float64  `json:"actual_yield_kg" validate:"required,gt=0"`
	ActualPricePerKg    *float64 `json:"actual_price_per_kg" validate:"omitempty,gte=0"`
	PaymentReceivedDate *string  `json:"payment_received_date" validate:"omitempty,datetime=2006-01-02"`
}

// How far this cooperative's own harvests have moved the model for one variety.
//
// OffsetDays is what the recorded harvests say on their own; AppliedOffsetDays
// is what the predictor actually uses, after shrinking the estimate toward the
// base model in proportion to how few observations back it. Both are reported,
// because the gap between them IS the honesty: two harvests do not get to move
// a prediction as far as twenty do.
type CalibrationResponse struct {
	VarietyID         string  `json:"variety_id"`
	VarietyName       string  `json:"variety_name"`
	CommodityName     string  `json:"commodity_name"`
	OffsetDays        float64 `json:"offset_days"`
	AppliedOffsetDays float64 `json:"applied_offset_days"`
	NObservations     int     `json:"n_observations"`
	ResidualSd        float64 `json:"residual_sd"`
}

type RecordHarvestResponse struct {
	BlockID string `json:"block_id"`
	PlotID  string `json:"plot_id"`
	// Nil when the recorded harvest was the first for its variety and there is
	// nothing yet to say about the model having learned anything.
	Calibration *CalibrationResponse `json:"calibration"`
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
	// Nil once GddAccumulated already clears GddRequired within known
	// weather. Otherwise the ISO date CumulativeGdd first turns from an
	// observed/forecast reading into a climatology projection.
	ProjectedFrom *string `json:"projected_from,omitempty"`
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

type WeekPriceResponse struct {
	PricePerKg float64 `json:"price_per_kg"`
	WeekStart  string  `json:"week_start"`
}

// The market reference for a block that is still growing.
//
// Deliberately not one number. Latest is this week's level; Seasonal is the
// same week of the year the harvest window opens, a year back. A farmer
// deciding when to sell needs the pair -- the seasonal figure says whether the
// window lands in a glut, and the latest says what that is worth today.
type PriceBenchmarkResponse struct {
	Latest WeekPriceResponse `json:"latest"`
	// Nil when the panel published no matching week; the screen says so rather
	// than falling back to Latest and quietly presenting today as the forecast.
	Seasonal *WeekPriceResponse `json:"seasonal"`
	Source   string             `json:"source"`
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
	FromPlan       bool                   `json:"from_plan"`
	Window         *HarvestWindowResponse `json:"window"`
	ExpectedTonnes *float64               `json:"expected_tonnes"`
	// Nil when the plot's province has no panel for this commodity. Most of
	// Indonesia does not yet: only Jawa Barat is seeded.
	Price *PriceBenchmarkResponse `json:"price"`
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
