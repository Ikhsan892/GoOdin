package event

import (
	"context"

	"goodin/pkg/cqrs"
)

// OnOrderCompletedHandler is the input port for the orders.order.completed event.
// Implementation: use_cases/loyalty/event/on_order_completed_handler.go
type OnOrderCompletedHandler interface {
	Handle(ctx context.Context, envelope cqrs.EventEnvelope) error
}
