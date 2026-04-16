package http

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	"goodin/drivers/http/api"
	core "goodin/internal"
)

// EchoRoute is implemented by FX-managed controllers.
// Add a Register(*echo.Group) method to your controller — no import of this package
// needed (Go structural typing); annotate via AsEchoRoute when providing to FX.
type EchoRoute interface {
	Register(g *echo.Group)
}

// AsEchoRoute annotates a controller constructor so FX collects it
// into the group:"echo_routes" value group consumed by NewEchoFX.
//
// Usage in cmd/:
//
//	fx.Provide(httpdriver.AsEchoRoute(api.NewLoyaltyController))
func AsEchoRoute(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(EchoRoute)),
		fx.ResultTags(`group:"echo_routes"`),
	)
}

type echoFXParams struct {
	fx.In

	LC     fx.Lifecycle
	App    core.App
	Routes []EchoRoute `group:"echo_routes"`
}

// NewEchoFX is the FX-compatible Echo provider.
// It delegates to NewEcho for the full setup (middleware, CORS, request logging,
// swagger, legacy routes via api.InitRoutes), then registers FX-provided routes
// into /api and wires start/stop into the FX lifecycle.
func NewEchoFX(p echoFXParams) *echo.Echo {
	adapter := NewEcho(p.App) // full middleware + legacy route setup unchanged
	e := adapter.ec

	// FX-provided routes (e.g., LoyaltyController) registered into /api
	prefix := e.Group("/api")

	for _, route := range p.Routes {
		route.Register(prefix)
	}

	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return adapter.Init()
		},
		OnStop: func(ctx context.Context) error {
			return adapter.Close()
		},
	})

	return e
}

// Module is the ready-to-use FX module. Include it in fx.New().
// It bundles the Echo server and all domain controllers — cmd/ only needs this one line.
var Module = fx.Module("echo_http",
	fx.Provide(
		NewEchoFX,

		// ── Controllers ───────────────────────────────────────────────────────
		// Register new domain controllers here. Each one is collected into
		// group:"echo_routes" and registered into /api by NewEchoFX.
		AsEchoRoute(api.NewLoyaltyController),
	),
)
