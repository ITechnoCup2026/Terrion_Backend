package weather

import (
	"math"

	"terrion-backend/internal/constants"
)

type GridCell struct {
	GridLat float64
	GridLng float64
}

func SnapToGrid(lat, lng float64) GridCell {
	return GridCell{
		GridLat: withoutNegativeZero(math.Round(lat/constants.GridStep) * constants.GridStep),
		GridLng: withoutNegativeZero(math.Round(lng/constants.GridStep) * constants.GridStep),
	}
}

func withoutNegativeZero(value float64) float64 {
	if value == 0 {
		return 0
	}
	return value
}
