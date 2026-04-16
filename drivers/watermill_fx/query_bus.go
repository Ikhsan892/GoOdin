package watermillfx

import (
	"context"
	"fmt"
	"sync"

	"goodin/pkg/cqrs"
)

var _ cqrs.QueryBus = (*queryBus)(nil)

type queryHandlerFunc func(ctx context.Context, query cqrs.Query) (any, error)

type queryBus struct {
	mu       sync.RWMutex
	handlers map[string]queryHandlerFunc
}

func newQueryBus() *queryBus {
	return &queryBus{handlers: make(map[string]queryHandlerFunc)}
}

// Register maps a query name to a handler function. Call from HandlerRegistrar.Register.
func (b *queryBus) Register(queryName string, fn queryHandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[queryName] = fn
}

// RegisterHandler maps a query name to a handler callable from outside the package.
func (b *queryBus) RegisterHandler(name string, fn func(context.Context, cqrs.Query) (any, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = queryHandlerFunc(fn)
}

func (b *queryBus) Ask(ctx context.Context, query cqrs.Query) (any, error) {
	b.mu.RLock()
	fn, ok := b.handlers[query.QueryName()]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler registered for query: %s", query.QueryName())
	}
	return fn(ctx, query)
}
