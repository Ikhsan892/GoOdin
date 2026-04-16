package query

import (
	"context"
	"log/slog"
	"time"

	core "goodin/internal"
	"goodin/use_cases/loyalty/dto"
	portquery "goodin/use_cases/loyalty/port/input/query"
	"goodin/use_cases/loyalty/port/output/cache"
	"goodin/use_cases/loyalty/port/output/repository"
)

var _ portquery.GetLoyaltyBalanceQueryHandler = (*GetLoyaltyBalanceHandler)(nil)

// GetLoyaltyBalanceHandler implements the get-loyalty-balance query.
// Strategy: try Redis cache first; on miss, fall back to the read repository.
// Queries are synchronous and never go through Watermill.
type GetLoyaltyBalanceHandler struct {
	app      core.App
	readRepo repository.LoyaltyReadRepository
	cache    cache.LoyaltyCacheRepository
}

func NewGetLoyaltyBalanceHandler(
	app core.App,
	readRepo repository.LoyaltyReadRepository,
	cacheRepo cache.LoyaltyCacheRepository,
) *GetLoyaltyBalanceHandler {
	return &GetLoyaltyBalanceHandler{
		app:      app,
		readRepo: readRepo,
		cache:    cacheRepo,
	}
}

func (h *GetLoyaltyBalanceHandler) Handle(ctx context.Context, q dto.GetLoyaltyBalanceRequest) (dto.GetLoyaltyBalanceResponse, error) {
	// 1. Cache lookup
	if h.cache != nil {
		if cached, ok, err := h.cache.GetBalance(ctx, q.CustomerID); err == nil && ok {
			return cached, nil
		}
	}

	// 2. Read repository fallback
	result, err := h.readRepo.GetBalanceByCustomerID(ctx, q.CustomerID)
	if err != nil {
		return dto.GetLoyaltyBalanceResponse{}, err
	}

	// 3. Populate cache for next call
	if h.cache != nil {
		if err := h.cache.SetBalance(ctx, q.CustomerID, result, 5*time.Minute); err != nil {
			h.app.Logger().Warn("loyalty: failed to cache balance",
				slog.String("customer_id", q.CustomerID),
				slog.String("error", err.Error()))
		}
	}

	return result, nil
}

// compile-time assertion that GetLoyaltyBalanceRequest implements Query
var _ interface{ QueryName() string } = (*dto.GetLoyaltyBalanceRequest)(nil)
