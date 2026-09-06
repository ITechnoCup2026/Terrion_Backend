package rdkk

import (
	"slices"

	"terrion-backend/internal/constants"
)

var orderTransitions = map[constants.OrderStatus][]constants.OrderStatus{
	constants.OrderDraft:     {constants.OrderSubmitted, constants.OrderCancelled},
	constants.OrderSubmitted: {constants.OrderCompleted, constants.OrderCancelled},
}

func CanTransitionOrder(from, to constants.OrderStatus) bool {
	return slices.Contains(orderTransitions[from], to)
}

func NextOrderStatuses(from constants.OrderStatus) []constants.OrderStatus {
	allowed := orderTransitions[from]
	next := make([]constants.OrderStatus, len(allowed))
	copy(next, allowed)
	return next
}
