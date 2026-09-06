package rdkk_test

import (
	"testing"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/rdkk"
)

func TestOrderTransitionsAllowsOneStepForward(t *testing.T) {
	steps := []struct{ from, to constants.OrderStatus }{
		{constants.OrderDraft, constants.OrderSubmitted},
		{constants.OrderDraft, constants.OrderCancelled},
		{constants.OrderSubmitted, constants.OrderCompleted},
		{constants.OrderSubmitted, constants.OrderCancelled},
	}
	for _, step := range steps {
		if !rdkk.CanTransitionOrder(step.from, step.to) {
			t.Errorf("CanTransitionOrder(%q, %q) = false, want true", step.from, step.to)
		}
	}
}

func TestOrderTransitionsRefusesSkippingSubmission(t *testing.T) {
	if rdkk.CanTransitionOrder(constants.OrderDraft, constants.OrderCompleted) {
		t.Error("draft -> completed should be refused: it skips ever being submitted")
	}
}

func TestOrderTransitionsRefusesGoingBackwards(t *testing.T) {
	steps := []struct{ from, to constants.OrderStatus }{
		{constants.OrderSubmitted, constants.OrderDraft},
		{constants.OrderCompleted, constants.OrderDraft},
		{constants.OrderCompleted, constants.OrderSubmitted},
		{constants.OrderCancelled, constants.OrderDraft},
	}
	for _, step := range steps {
		if rdkk.CanTransitionOrder(step.from, step.to) {
			t.Errorf("CanTransitionOrder(%q, %q) = true, want false: nothing moves backwards",
				step.from, step.to)
		}
	}
}

func TestOrderTransitionsRefusesLeavingAFinalStatus(t *testing.T) {
	finals := []constants.OrderStatus{constants.OrderCompleted, constants.OrderCancelled}
	targets := []constants.OrderStatus{
		constants.OrderDraft, constants.OrderSubmitted,
		constants.OrderCompleted, constants.OrderCancelled,
	}
	for _, from := range finals {
		for _, to := range targets {
			if rdkk.CanTransitionOrder(from, to) {
				t.Errorf("CanTransitionOrder(%q, %q) = true, want false: %q is final", from, to, from)
			}
		}
	}
}

func TestOrderTransitionsRefusesStandingStill(t *testing.T) {
	all := []constants.OrderStatus{
		constants.OrderDraft, constants.OrderSubmitted,
		constants.OrderCompleted, constants.OrderCancelled,
	}
	for _, status := range all {
		if rdkk.CanTransitionOrder(status, status) {
			t.Errorf("CanTransitionOrder(%q, %q) = true, want false: standing still is not a transition",
				status, status)
		}
	}
}

func TestNextOrderStatusesIsNeverNil(t *testing.T) {
	all := []constants.OrderStatus{
		constants.OrderDraft, constants.OrderSubmitted,
		constants.OrderCompleted, constants.OrderCancelled,
	}
	for _, status := range all {
		if rdkk.NextOrderStatuses(status) == nil {
			t.Errorf("NextOrderStatuses(%q) = nil, want an empty slice", status)
		}
	}
}
