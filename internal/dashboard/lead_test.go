package dashboard_test

import (
	"testing"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/dashboard"
)

func flagged(isoWeek string, tonnes float64, plotCount int) dashboard.CollisionWeek {
	return dashboard.CollisionWeek{
		FlaggedWeek: agronomy.FlaggedWeek{
			WeekBucket: agronomy.WeekBucket{ISOWeek: isoWeek, Tonnes: tonnes},
		},
		PlotCount: plotCount,
	}
}

func TestSelectLeadCollisionOfNothingIsNil(t *testing.T) {
	if lead := dashboard.SelectLeadCollision(nil); lead != nil {
		t.Errorf("lead = %+v, want nil", lead)
	}
}

func TestSelectLeadCollisionPrefersAPileUpOverAHeavierSinglePlotWeek(t *testing.T) {
	lead := dashboard.SelectLeadCollision([]dashboard.CollisionWeek{
		flagged("kentang", 44.7, 1),
		flagged("padi", 36.7, 20),
	})

	if lead == nil || lead.ISOWeek != "padi" {
		t.Errorf("lead = %+v, want the multi-plot week", lead)
	}
}

func TestSelectLeadCollisionTakesTheHeaviestPileUp(t *testing.T) {
	lead := dashboard.SelectLeadCollision([]dashboard.CollisionWeek{
		flagged("padi-late", 35.6, 20),
		flagged("padi-early", 36.7, 20),
		flagged("cabai", 15.8, 2),
	})

	if lead == nil || lead.ISOWeek != "padi-early" {
		t.Errorf("lead = %+v, want padi-early", lead)
	}
}

func TestSelectLeadCollisionFallsBackToASinglePlotWeek(t *testing.T) {
	lead := dashboard.SelectLeadCollision([]dashboard.CollisionWeek{
		flagged("kentang", 44.7, 1),
		flagged("wortel", 15.2, 1),
	})

	if lead == nil || lead.ISOWeek != "kentang" {
		t.Errorf("lead = %+v, want kentang", lead)
	}
}

func TestSelectLeadCollisionBreaksATonnageTieByPlotCount(t *testing.T) {
	lead := dashboard.SelectLeadCollision([]dashboard.CollisionWeek{
		flagged("a", 20, 3),
		flagged("b", 20, 9),
	})

	if lead == nil || lead.ISOWeek != "b" {
		t.Errorf("lead = %+v, want b", lead)
	}
}

func TestSelectLeadCollisionTreatsTwoPlotsAsAPileUp(t *testing.T) {
	lead := dashboard.SelectLeadCollision([]dashboard.CollisionWeek{
		flagged("single", 100, 1),
		flagged("pair", 10, 2),
	})

	if lead == nil || lead.ISOWeek != "pair" {
		t.Errorf("lead = %+v, want pair", lead)
	}
}
