package agronomy_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
)

var predictedMid = time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

func lateBy(days int) agronomy.CalibrationObservation {
	return agronomy.CalibrationObservation{
		PredictedMid: predictedMid,
		Actual:       agronomy.AddDays(predictedMid, days),
	}
}

func TestFitCalibrationWithNoObservationsIsZero(t *testing.T) {
	got := agronomy.FitCalibration(nil)
	if got != (agronomy.Calibration{}) {
		t.Errorf("FitCalibration(nil) = %+v, want zero value", got)
	}
}

func TestFitCalibrationRecoversConsistentBias(t *testing.T) {
	got := agronomy.FitCalibration([]agronomy.CalibrationObservation{
		lateBy(5), lateBy(5), lateBy(5), lateBy(5),
	})

	closeTo(t, "OffsetDays", got.OffsetDays, 5, 5e-6)
	closeTo(t, "ResidualSd", got.ResidualSd, 0, 5e-6)
	if got.NObservations != 4 {
		t.Errorf("NObservations = %d, want 4", got.NObservations)
	}
}

func TestFitCalibrationReportsSpreadAsResidualSd(t *testing.T) {
	got := agronomy.FitCalibration([]agronomy.CalibrationObservation{
		lateBy(3), lateBy(5), lateBy(7),
	})

	closeTo(t, "OffsetDays", got.OffsetDays, 5, 5e-6)
	closeTo(t, "ResidualSd", got.ResidualSd, 2, 5e-6)
}

func TestFitCalibrationRecoversNegativeBias(t *testing.T) {
	got := agronomy.FitCalibration([]agronomy.CalibrationObservation{lateBy(-4), lateBy(-6)})

	closeTo(t, "OffsetDays", got.OffsetDays, -5, 5e-6)
}

func TestShrunkOffsetPullsSingleObservationTowardZero(t *testing.T) {
	fitted := agronomy.FitCalibration([]agronomy.CalibrationObservation{lateBy(8)})

	closeTo(t, "ShrunkOffset", agronomy.ShrunkOffset(fitted), 2, 5e-6)
}

func TestShrunkOffsetBarelyMovesALargeSample(t *testing.T) {
	observations := make([]agronomy.CalibrationObservation, 97)
	for i := range observations {
		observations[i] = lateBy(8)
	}
	fitted := agronomy.FitCalibration(observations)

	closeTo(t, "ShrunkOffset", agronomy.ShrunkOffset(fitted), 7.76, 0.05)
}
