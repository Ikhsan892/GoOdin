package cqrs

import (
	"context"
	"time"
)

// DomainEvent is a marker interface for all domain event structs.
// Implementations live in use_cases/{domain}/domain/event/.
type DomainEvent interface {
	// EventName returns the topic string (must match a constant in pkg/events/).
	EventName() string

	// OccurredAt returns the time the event was raised within the domain.
	OccurredAt() time.Time

	// AggregateID returns the ID of the aggregate that raised this event.
	AggregateID() string
}

// EventHandler handles a single domain event type delivered via EventEnvelope.
// Implementations live in use_cases/{domain}/event/.
// Each handler must be idempotent — it may be called more than once for the same event
// if the app restarts before the message is acked (Watermill SQL at-least-once delivery).
type EventHandler interface {
	Handle(ctx context.Context, envelope EventEnvelope) error
}
