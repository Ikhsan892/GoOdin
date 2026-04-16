package cqrs

import "context"

// Command is a marker interface for all command structs.
// Command structs are defined in use_cases/{domain}/dto/ and implement this interface.
// A command expresses the intent to change state (write side of CQRS).
type Command interface {
	// CommandName returns a human-readable identifier used for logging and routing.
	CommandName() string
}

// CommandHandler handles a specific command type and returns a result.
// Type parameter C must implement Command; R is the response type.
// Implementations live in use_cases/{domain}/command/.
//
// Usage:
//
//	type CreditLoyaltyHandler struct { ... }
//	var _ cqrs.CommandHandler[dto.CreditLoyaltyRequest, dto.CreditLoyaltyResponse] = (*CreditLoyaltyHandler)(nil)
type CommandHandler[C Command, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}
