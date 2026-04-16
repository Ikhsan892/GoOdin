package query

import (
	"context"

	"goodin/use_cases/loyalty/dto"
)

// GetLoyaltyBalanceQueryHandler is the input port for the get-balance query.
// Implementation: use_cases/loyalty/query/get_loyalty_balance_handler.go
type GetLoyaltyBalanceQueryHandler interface {
	Handle(ctx context.Context, query dto.GetLoyaltyBalanceRequest) (dto.GetLoyaltyBalanceResponse, error)
}
