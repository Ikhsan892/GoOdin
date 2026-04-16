package watermill

import (
	"context"
	"fmt"
	"sync"

	"goodin/pkg/cqrs"
)

var _ cqrs.CommandBus = (*commandBus)(nil)

type commandHandlerFunc func(ctx context.Context, cmd cqrs.Command) (any, error)

// commandBus is a synchronous in-memory command dispatcher.
// Commands are handled synchronously (unlike events which go through Watermill).
// Handlers are registered once at startup via Register().
type commandBus struct {
	mu       sync.RWMutex
	handlers map[string]commandHandlerFunc
}

func newCommandBus() *commandBus {
	return &commandBus{handlers: make(map[string]commandHandlerFunc)}
}

// Register maps a command name to a handler function.
// Called from drivers/watermill/register.go during Init().
func (b *commandBus) Register(commandName string, fn commandHandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[commandName] = fn
}

// Dispatch invokes the handler registered for cmd.CommandName().
// Returns (any, error) — callers type-assert the result.
func (b *commandBus) Dispatch(ctx context.Context, cmd cqrs.Command) (any, error) {
	b.mu.RLock()
	fn, ok := b.handlers[cmd.CommandName()]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler registered for command: %s", cmd.CommandName())
	}
	return fn(ctx, cmd)
}
