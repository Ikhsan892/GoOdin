package repositories

import (
	"context"
	"time"

	"goodin/use_cases/loyalty/dto"
	"goodin/use_cases/loyalty/port/output/cache"
)

var _ cache.LoyaltyCacheRepository = (*noopLoyaltyCacheRepository)(nil)

// noopLoyaltyCacheRepository is a pass-through implementation that never caches.
// Replace with a Redis implementation when a cache layer is available.
type noopLoyaltyCacheRepository struct{}

func NewNoopLoyaltyCacheRepository() cache.LoyaltyCacheRepository {
	return &noopLoyaltyCacheRepository{}
}

func (r *noopLoyaltyCacheRepository) SetBalance(_ context.Context, _ string, _ dto.GetLoyaltyBalanceResponse, _ time.Duration) error {
	return nil
}

func (r *noopLoyaltyCacheRepository) GetBalance(_ context.Context, _ string) (dto.GetLoyaltyBalanceResponse, bool, error) {
	return dto.GetLoyaltyBalanceResponse{}, false, nil
}

func (r *noopLoyaltyCacheRepository) InvalidateBalance(_ context.Context, _ string) error {
	return nil
}
