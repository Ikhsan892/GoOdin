package repository

import (
	"context"

	"goodin/use_cases/loyalty/domain/aggregate"
)

// LoyaltyWriteRepository is the output port for persisting the LoyaltyAggregate.
// Works with aggregates (write side of CQRS).
// Implementation: repositories/loyalty/loyalty_write_postgres_repository.go
type LoyaltyWriteRepository interface {
	// Save persists the full aggregate state. Upserts by ID.
	Save(ctx context.Context, agg *aggregate.LoyaltyAggregate) error

	// FindByCustomerID loads the aggregate for a customer.
	// Returns (nil, nil) if not found — callers should create a new aggregate.
	FindByCustomerID(ctx context.Context, customerID string) (*aggregate.LoyaltyAggregate, error)
}
