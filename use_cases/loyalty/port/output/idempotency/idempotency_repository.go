package idempotency

import "context"

// IdempotencyRepository tracks which domain events have already been processed.
// This prevents double-processing when Watermill replays unacked messages after a crash.
//
// Key: the CausationEventID from the EventEnvelope (the EventID that triggered the handler).
// Implementation: repositories/loyalty/loyalty_idempotency_postgres_repository.go
type IdempotencyRepository interface {
	// Exists returns true if the given causation event ID has already been processed.
	Exists(ctx context.Context, causationEventID string) (bool, error)

	// Mark records the causation event ID as processed.
	// Must be called inside the same DB transaction as the business work.
	Mark(ctx context.Context, causationEventID string) error
}
