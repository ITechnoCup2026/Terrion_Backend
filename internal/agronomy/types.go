package agronomy

import (
	"time"

	"terrion-backend/internal/constants"
)

type TempDay struct {
	Date string
	TMin float64
	TMax float64
}

type ClimateNormal struct {
	DayOfYear int
	MeanC     float64
	SdC       float64
}

type Variety struct {
	GddRequirement   float64
	BaseTempC        float64
	DaysToHarvestMin int
	DaysToHarvestMax int
	YieldPerHaMin    float64
	YieldPerHaMax    float64
}

type Calibration struct {
	OffsetDays    float64
	NObservations int
	ResidualSd    float64
}

type CumulativeGdd struct {
	Date string
	Gdd  float64
}

type DateRange struct {
	Start time.Time
	End   time.Time
}

type HarvestWindow struct {
	Start          time.Time
	End            time.Time
	Confidence     float64
	GddAccumulated float64
	GddRequired    float64
	Stage          constants.GrowthStage
	Basis          constants.WindowBasis
	Plausibility   constants.Plausibility
	CumulativeGdd  []CumulativeGdd
}

type BlockProjection struct {
	BlockID        string
	PlotID         string
	CommodityID    string
	Window         DateRange
	ExpectedTonnes float64
}
