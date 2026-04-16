package cache

import (
	"context"
	"time"

	"goodin/use_cases/loyalty/dto"
)

// LoyaltyCacheRepository is the output port for caching loyalty query results in Redis.
// Implementation: repositories/loyalty/loyalty_cache_redis_repository.go
type LoyaltyCacheRepository interface {
	SetBalance(ctx context.Context, customerID string, balance dto.GetLoyaltyBalanceResponse, ttl time.Duration) error
	GetBalance(ctx context.Context, customerID string) (dto.GetLoyaltyBalanceResponse, bool, error)
	InvalidateBalance(ctx context.Context, customerID string) error
}
