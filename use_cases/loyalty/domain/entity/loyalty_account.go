package entity

import (
	valueobject "goodin/use_cases/loyalty/domain/value_object"
)

// LoyaltyAccount is the core domain entity for a customer's loyalty record.
// It has identity (ID) and mutable state.
// Business logic is enforced by LoyaltyAggregate — do not mutate fields directly.
type LoyaltyAccount struct {
	ID         string
	CustomerID string
	Balance    valueobject.Points
	Tier       valueobject.Tier
	CreatedAt  int64 // Unix timestamp
	UpdatedAt  int64 // Unix timestamp
}
