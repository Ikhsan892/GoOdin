package cqrs

import "context"

// Query is a marker interface for all query structs.
// A query expresses the intent to read state (read side of CQRS).
// Queries are always synchronous — they never go through Watermill.
type Query interface {
	// QueryName returns a human-readable identifier used for logging.
	QueryName() string
}

// QueryHandler handles a specific query type and returns a result.
// Type parameter Q must implement Query; R is the response type.
// Implementations live in use_cases/{domain}/query/.
type QueryHandler[Q Query, R any] interface {
	Handle(ctx context.Context, query Q) (R, error)
}
