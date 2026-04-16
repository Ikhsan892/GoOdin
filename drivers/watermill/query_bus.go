package watermill

import (
	"context"
	"fmt"
	"sync"

	"goodin/pkg/cqrs"
)

var _ cqrs.QueryBus = (*queryBus)(nil)

type queryHandlerFunc func(ctx context.Context, query cqrs.Query) (any, error)

// queryBus is a synchronous in-memory query dispatcher.
// Queries never go through Watermill — they are read-only and synchronous.
type queryBus struct {
	mu       sync.RWMutex
	handlers map[string]queryHandlerFunc
}

func newQueryBus() *queryBus {
	return &queryBus{handlers: make(map[string]queryHandlerFunc)}
}

// Register maps a query name to a handler function.
func (b *queryBus) Register(queryName string, fn queryHandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[queryName] = fn
}

// Ask invokes the handler registered for query.QueryName().
func (b *queryBus) Ask(ctx context.Context, query cqrs.Query) (any, error) {
	b.mu.RLock()
	fn, ok := b.handlers[query.QueryName()]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler registered for query: %s", query.QueryName())
	}
	return fn(ctx, query)
}
