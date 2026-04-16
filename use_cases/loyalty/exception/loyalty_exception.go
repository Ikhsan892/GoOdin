package exception

import "errors"

var (
	ErrLoyaltyAccountNotFound  = errors.New("loyalty account not found")
	ErrInsufficientPoints      = errors.New("insufficient loyalty points")
	ErrLoyaltyCreditFailed     = errors.New("failed to credit loyalty points")
	ErrDuplicateEvent          = errors.New("event already processed (idempotency)")
)
