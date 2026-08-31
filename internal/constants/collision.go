package constants

const MedianCapacityMultiplier = 2.5

var StaggerShiftCandidateDays = [...]int{7, 10, 14, -7, -10, -14}

type ThresholdBasis string

const (
	ThresholdCapacity ThresholdBasis = "capacity"
	ThresholdMedian   ThresholdBasis = "median"
)

type RefusalReason string

const (
	RefusedAlreadyPlanted RefusalReason = "already-planted"
	RefusedWouldBeInPast  RefusalReason = "would-be-in-the-past"
	RefusedNoShift        RefusalReason = "no-shift"
)
