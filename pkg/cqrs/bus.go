package cqrs

import "context"

// CommandBus dispatches a command to its single registered handler.
// Returns (any, error) — callers type-assert the result to the expected response type.
type CommandBus interface {
	Dispatch(ctx context.Context, cmd Command) (any, error)
}

// RegisterableCommandBus extends CommandBus with handler registration.
// Used by HandlerRegistrar.Register to wire handlers at startup.
type RegisterableCommandBus interface {
	CommandBus
	RegisterHandler(name string, fn func(context.Context, Command) (any, error))
}

// QueryBus dispatches a query synchronously to its single registered handler.
// Queries are read-only, synchronous, and never go through Watermill.
type QueryBus interface {
	Ask(ctx context.Context, query Query) (any, error)
}

// RegisterableQueryBus extends QueryBus with handler registration.
// Used by HandlerRegistrar.Register to wire handlers at startup.
type RegisterableQueryBus interface {
	QueryBus
	RegisterHandler(name string, fn func(context.Context, Query) (any, error))
}

// EventBus publishes domain events and routes them to subscribed handlers.
type EventBus interface {
	Publish(ctx context.Context, envelopes ...EventEnvelope) error
	Subscribe(eventName string, handler EventHandler) error
}

// HandlerRegistrar is implemented by domain packages that wire command, query,
// and event handlers into the buses at startup.
// In the FX world, provide implementations via watermillfx.AsHandler.
type HandlerRegistrar interface {
	Register(cmd RegisterableCommandBus, query RegisterableQueryBus, evt EventBus)
}
