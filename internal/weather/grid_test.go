package weather_test

import (
	"math"
	"testing"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/weather"
)

func TestGridStepIsAQuarterDegree(t *testing.T) {
	if constants.GridStep != 0.25 {
		t.Errorf("GridStep = %v, want 0.25", constants.GridStep)
	}
}

func TestSnapToGrid(t *testing.T) {
	tests := []struct {
		name    string
		lat     float64
		lng     float64
		wantLat float64
		wantLng float64
	}{
		{"nearest quarter degree", -7.21, 107.80, -7.25, 107.75},
		{"already snapped", -7.25, 107.75, -7.25, 107.75},
		{"negative half away from zero", -7.125, 0, -7.25, 0},
		{"negative half away from zero, larger", -7.375, 0, -7.5, 0},
		{"negative half below one", -0.125, 0, -0.25, 0},
		{"positive half away from zero", 7.125, 0, 7.25, 0},
		{"positive half below one", 0.125, 0, 0.25, 0},
	}

	for _, test := range tests {
		got := weather.SnapToGrid(test.lat, test.lng)
		if got.GridLat != test.wantLat || got.GridLng != test.wantLng {
			t.Errorf("%s: SnapToGrid(%v, %v) = %+v, want {%v %v}",
				test.name, test.lat, test.lng, got, test.wantLat, test.wantLng)
		}
	}
}

func TestSnapToGridNeverProducesNegativeZero(t *testing.T) {
	got := weather.SnapToGrid(-0.1, -0.05)

	if math.Signbit(got.GridLat) {
		t.Errorf("GridLat is negative zero, want positive zero")
	}
	if math.Signbit(got.GridLng) {
		t.Errorf("GridLng is negative zero, want positive zero")
	}
}
