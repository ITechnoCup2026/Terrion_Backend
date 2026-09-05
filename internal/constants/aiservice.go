package constants

import "time"

type PlanEngine string

const (
	PlanEngineAIService PlanEngine = "ai-service"
	PlanEngineFallback  PlanEngine = "fallback"
)

const (
	AIContractVersion = "1.0"
	AIContractMajor   = "1"
	AIProposePath     = "/v1/plan/propose"
	AIHealthPath      = "/health"
)

const (
	AIRetryBackoff    = 250 * time.Millisecond
	AIRetryJitter     = 100 * time.Millisecond
	AIBreakerTrip     = 3
	AIBreakerCooldown = 60 * time.Second
)

const (
	AIPlanCacheKey = "terrion:ai:plan:v1:"
	AIPlanCacheTTL = 6 * time.Hour
)

const (
	AIMaxCandidates = 2000
	AIMaxDemandRows = 400
)
