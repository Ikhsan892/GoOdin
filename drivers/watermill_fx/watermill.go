// Package watermillfx provides an Uber FX module for the Watermill event bus.
//
// It exposes CommandBus, QueryBus, and EventBus as injectable FX values,
// manages the Watermill router lifecycle (start/stop) via fx.Lifecycle,
// and bundles all domain HandlerRegistrars so cmd/ needs only one line.
//
// Usage in an FX app:
//
//	fx.New(
//	    watermillfx.Module,
//	)
package watermillfx

import (
	"context"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
	"go.uber.org/fx"

	"goodin/pkg/cqrs"
	loyalty "goodin/use_cases/loyalty"
)

// AsHandler annotates a cqrs.HandlerRegistrar constructor so FX collects it
// into the group:"watermill_handlers" value group.
func AsHandler(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(cqrs.HandlerRegistrar)),
		fx.ResultTags(`group:"watermill_handlers"`),
	)
}

// Params holds all dependencies injected by FX.
type Params struct {
	fx.In

	LC         fx.Lifecycle
	Log        *slog.Logger
	Registrars []cqrs.HandlerRegistrar `group:"watermill_handlers"`
}

// Buses holds the values provided to the FX container.
type Buses struct {
	fx.Out

	CommandBus cqrs.CommandBus
	QueryBus   cqrs.QueryBus
	EventBus   cqrs.EventBus
}

// New constructs the Watermill router and buses, registers domain handlers,
// and appends OnStart/OnStop hooks to the FX lifecycle.
func New(p Params) (Buses, error) {
	logger := newLogger(p.Log)

	router, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return Buses{}, err
	}

	cmdBus := newCommandBus()
	qBus := newQueryBus()
	evtBus := newEventBus(router, logger)

	for _, r := range p.Registrars {
		r.Register(cmdBus, qBus, evtBus)
	}

	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := router.Run(ctx); err != nil {
					p.Log.Error("watermill router stopped", slog.String("error", err.Error()))
				}
			}()
			p.Log.Info("> Watermill event bus started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return router.Close()
		},
	})

	return Buses{
		CommandBus: cmdBus,
		QueryBus:   qBus,
		EventBus:   evtBus,
	}, nil
}

// Module is the ready-to-use FX module. Include it in fx.New().
// It bundles the router, all buses, and all domain registrars —
// cmd/ only needs this one line.
var Module = fx.Module("watermill",
	fx.Provide(
		New,
		AsHandler(loyalty.NewRegistrar),
	),
)
