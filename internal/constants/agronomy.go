package constants

const ISODateLayout = "2006-01-02"

type GrowthStage int

const (
	StageBare GrowthStage = iota
	StageEstablished
	StageVegetative
	StageRipening
	StageReady
)

const (
	EstablishedGddFraction = 0.15
	VegetativeGddFraction  = 0.5
	RipeningGddFraction    = 0.85
)

type WindowBasis string

const (
	BasisObserved    WindowBasis = "observed"
	BasisForecast    WindowBasis = "forecast"
	BasisClimatology WindowBasis = "climatology"
)

type Plausibility string

const (
	PlausibilityOk          Plausibility = "ok"
	PlausibilityEarly       Plausibility = "early"
	PlausibilityLate        Plausibility = "late"
	PlausibilityImplausible Plausibility = "implausible"
)
