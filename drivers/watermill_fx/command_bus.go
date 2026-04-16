package watermillfx

import (
	"context"
	"fmt"
	"sync"

	"goodin/pkg/cqrs"
)

var _ cqrs.CommandBus = (*commandBus)(nil)

type commandHandlerFunc func(ctx context.Context, cmd cqrs.Command) (any, error)

type commandBus struct {
	mu       sync.RWMutex
	handlers map[string]commandHandlerFunc
}

func newCommandBus() *commandBus {
	return &commandBus{handlers: make(map[string]commandHandlerFunc)}
}

// Register maps a command name to a handler function. Call from HandlerRegistrar.Register.
func (b *commandBus) Register(commandName string, fn commandHandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[commandName] = fn
}

// RegisterHandler maps a command name to a handler callable from outside the package.
func (b *commandBus) RegisterHandler(name string, fn func(context.Context, cqrs.Command) (any, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = commandHandlerFunc(fn)
}

func (b *commandBus) Dispatch(ctx context.Context, cmd cqrs.Command) (any, error) {
	b.mu.RLock()
	fn, ok := b.handlers[cmd.CommandName()]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler registered for command: %s", cmd.CommandName())
	}
	return fn(ctx, cmd)
}
