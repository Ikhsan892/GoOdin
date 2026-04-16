package command

import (
	"context"

	"goodin/use_cases/loyalty/dto"
)

// CreditLoyaltyCommandHandler is the input port for the credit loyalty command.
// Implementation: use_cases/loyalty/command/credit_loyalty_handler.go
type CreditLoyaltyCommandHandler interface {
	Handle(ctx context.Context, cmd dto.CreditLoyaltyRequest) (dto.CreditLoyaltyResponse, error)
}
