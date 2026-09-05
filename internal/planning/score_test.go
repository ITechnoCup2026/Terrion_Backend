package planning_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/planning"
)

func mustPlanDate(t *testing.T, iso string) time.Time {
	t.Helper()

	parsed, err := agronomy.UTCDate(iso)
	if err != nil {
		t.Fatalf("parsing %q: %v", iso, err)
	}
	return parsed
}

func assignmentAt(
	t *testing.T, plotID, commodityID, start, end string, tonnesMid, tonnesHigh float64,
) planning.Assignment {
	t.Helper()

	return planning.Assignment{
		PlotID:       plotID,
		PlotName:     "Lahan " + plotID,
		MemberID:     "member-" + plotID,
		MemberName:   "Anggota " + plotID,
		AreaHa:       1,
		CommodityID:  commodityID,
		VarietyID:    "variety-" + commodityID,
		VarietyName:  "Varietas " + commodityID,
		PlantingDate: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC),
		Window: agronomy.DateRange{
			Start: mustPlanDate(t, start),
			End:   mustPlanDate(t, end),
		},
		Plausibility: constants.PlausibilityOk,
		TonnesLow:    tonnesMid * 0.8,
		TonnesMid:    tonnesMid,
		TonnesHigh:   tonnesHigh,
	}
}

func TestMeasureAddsTonnageLandingInTheSameWeek(t *testing.T) {
	metrics := planning.Measure([]planning.Assignment{
		assignmentAt(t, "a", "padi", "2027-03-01", "2027-03-07", 10, 12),
		assignmentAt(t, "b", "padi", "2027-03-01", "2027-03-07", 6, 7),
	}, nil, nil)

	if metrics.PeakTonnesExpected < 15.9 || metrics.PeakTonnesExpected > 16.1 {
		t.Errorf("PeakTonnesExpected = %v, want about 16", metrics.PeakTonnesExpected)
	}
	if metrics.TotalTonnesMid != 16 {
		t.Errorf("TotalTonnesMid = %v, want 16", metrics.TotalTonnesMid)
	}
}

func TestMeasureWorstPeakIsNeverBelowExpectedPeak(t *testing.T) {
	metrics := planning.Measure([]planning.Assignment{
		assignmentAt(t, "a", "padi", "2027-03-01", "2027-03-21", 10, 12),
		assignmentAt(t, "b", "padi", "2027-03-08", "2027-03-28", 6, 7),
	}, nil, nil)

	if metrics.PeakTonnesWorst < metrics.PeakTonnesExpected {
		t.Errorf("PeakTonnesWorst = %v, want >= PeakTonnesExpected %v",
			metrics.PeakTonnesWorst, metrics.PeakTonnesExpected)
	}
}

func TestMeasureSpreadingHarvestsLowersThePeak(t *testing.T) {
	together := planning.Measure([]planning.Assignment{
		assignmentAt(t, "a", "padi", "2027-03-01", "2027-03-07", 10, 12),
		assignmentAt(t, "b", "padi", "2027-03-01", "2027-03-07", 10, 12),
	}, nil, nil)

	apart := planning.Measure([]planning.Assignment{
		assignmentAt(t, "a", "padi", "2027-03-01", "2027-03-07", 10, 12),
		assignmentAt(t, "b", "padi", "2027-03-15", "2027-03-21", 10, 12),
	}, nil, nil)

	if apart.PeakTonnesExpected >= together.PeakTonnesExpected {
		t.Errorf("spread peak = %v, want below stacked peak %v",
			apart.PeakTonnesExpected, together.PeakTonnesExpected)
	}
}

func TestMeasureLeavesValueEmptyWhenAnyCommodityHasNoReferencePrice(t *testing.T) {
	metrics := planning.Measure([]planning.Assignment{
		assignmentAt(t, "a", "padi", "2027-03-01", "2027-03-07", 10, 12),
		assignmentAt(t, "b", "cabai", "2027-03-01", "2027-03-07", 2, 3),
	}, map[string]float64{"padi": 6500}, nil)

	if metrics.GrossValue != nil {
		t.Errorf("GrossValue = %v, want nil", *metrics.GrossValue)
	}
}

func TestMeasureValuesTonnageAtTheReferencePrice(t *testing.T) {
	metrics := planning.Measure([]planning.Assignment{
		assignmentAt(t, "a", "padi", "2027-03-01", "2027-03-07", 10, 12),
	}, map[string]float64{"padi": 6500}, nil)

	if metrics.GrossValue == nil {
		t.Fatal("GrossValue = nil, want a number")
	}
	if *metrics.GrossValue != 65000000 {
		t.Errorf("GrossValue = %v, want 65000000", *metrics.GrossValue)
	}
}

func TestMeasureCapsCoveredDemandAtWhatWasAsked(t *testing.T) {
	week := agronomy.ISOWeekKey(mustPlanDate(t, "2027-03-01"))

	metrics := planning.Measure([]planning.Assignment{
		assignmentAt(t, "a", "padi", "2027-03-01", "2027-03-07", 10, 12),
	}, nil, []planning.Demand{{CommodityID: "padi", ISOWeek: week, Kg: 3000}})

	if metrics.DemandCoveredKg != 3000 {
		t.Errorf("DemandCoveredKg = %v, want 3000", metrics.DemandCoveredKg)
	}
}
