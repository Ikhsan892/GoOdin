// Package watermill provides a Watermill-backed IDriver that exposes CommandBus,
// QueryBus, and EventBus to the rest of the application.
//
// Requires (add via go get):
//
//	go get github.com/ThreeDotsLabs/watermill
//	go get github.com/ThreeDotsLabs/watermill-sql/v2
package watermill

import (
	"log/slog"

	"goodin/pkg/cqrs"
	"goodin/pkg/driver"

	core "goodin/internal"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

var _ driver.IDriver = (*WatermillDriver)(nil)

const WATERMILL_DRIVER = "WATERMILL_DRIVER"

// WatermillDriver implements driver.IDriver and manages the Watermill router lifecycle.
// Retrieve from app.Driver().Instance(WATERMILL_DRIVER).(*WatermillDriver) to access buses.
type WatermillDriver struct {
	app        core.App
	router     *message.Router
	logger     watermill.LoggerAdapter
	commandBus *commandBus
	queryBus   *queryBus
	eventBus   *eventBus
}

// NewWatermillDriver creates a new WatermillDriver. Register with app.Driver().RunDriver().
func NewWatermillDriver(app core.App) *WatermillDriver {
	logger := newWatermillLogger(app.Logger())
	return &WatermillDriver{
		app:    app,
		logger: logger,
	}
}

// Name implements driver.IDriver.
func (w *WatermillDriver) Name() string { return WATERMILL_DRIVER }

// Init implements driver.IDriver. Builds and starts the Watermill router in a goroutine.
func (w *WatermillDriver) Init() error {
	router, err := message.NewRouter(message.RouterConfig{}, w.logger)
	if err != nil {
		return err
	}
	w.router = router

	// Build buses
	w.commandBus = newCommandBus()
	w.queryBus = newQueryBus()
	w.eventBus = newEventBus(w.app, w.router, w.logger)

	// Register all domain handlers (wires event subscriptions to watermill router)
	registerHandlers(w)

	// Start router in background — blocks until router.Close() is called
	go func() {
		if err := w.router.Run(w.app.Context()); err != nil {
			w.app.Logger().Error("watermill router stopped", slog.String("error", err.Error()))
		}
	}()

	w.app.Logger().Info("> Watermill event bus started")
	return nil
}

// Instance implements driver.IDriver. Returns the WatermillDriver itself.
// Usage: wm := app.Driver().Instance(WATERMILL_DRIVER).(*WatermillDriver)
func (w *WatermillDriver) Instance() interface{} { return w }

// Close implements driver.IDriver. Drains in-flight messages before returning.
func (w *WatermillDriver) Close() error {
	if w.router != nil {
		return w.router.Close()
	}
	return nil
}

// CommandBus returns the CommandBus for dispatching commands.
func (w *WatermillDriver) CommandBus() cqrs.CommandBus { return w.commandBus }

// QueryBus returns the QueryBus for asking queries.
func (w *WatermillDriver) QueryBus() cqrs.QueryBus { return w.queryBus }

// EventBus returns the EventBus for publishing and subscribing to domain events.
func (w *WatermillDriver) EventBus() cqrs.EventBus { return w.eventBus }
