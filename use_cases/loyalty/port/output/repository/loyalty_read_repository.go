package repository

import (
	"context"

	"goodin/use_cases/loyalty/dto"
)

// LoyaltyReadRepository is the output port for querying loyalty data (read side of CQRS).
// Returns flat DTOs optimized for API responses — no aggregate objects.
// Implementation: repositories/loyalty/loyalty_read_postgres_repository.go
type LoyaltyReadRepository interface {
	GetBalanceByCustomerID(ctx context.Context, customerID string) (dto.GetLoyaltyBalanceResponse, error)
}
