package watermill

// register.go is the EVENT TOPOLOGY MANIFEST for the entire application.
//
// ╔══════════════════════════════════════════════════════════════════════════════╗
// ║                     FULL EVENT TOPOLOGY (read this first)                   ║
// ╠══════════════════════════════════════════════════════════════════════════════╣
// ║  orders.order.completed                                                      ║
// ║    → [loyalty]   on_order_completed → credit points                         ║
// ║                    success: loyalty.account.credited                        ║
// ║                    failure: loyalty.account.credit_failed                   ║
// ║    → [notif]     on_order_completed → send confirmation email               ║
// ║                                                                              ║
// ║  loyalty.account.credited                                                   ║
// ║    → [notif]     on_loyalty_credited → send "you earned X points" email     ║
// ║    → [saga]      order_fulfillment_saga → wait for inventory too            ║
// ║                                                                              ║
// ║  loyalty.account.credit_failed                                              ║
// ║    → [orders]    on_loyalty_credit_failed → mark order as failed            ║
// ║                                                                              ║
// ║  orders.fulfillment.ready (emitted by saga when all prerequisites met)      ║
// ║    → [shipping]  on_fulfillment_ready → create shipment                     ║
// ║    → [notif]     on_fulfillment_ready → send "order packed" email           ║
// ╚══════════════════════════════════════════════════════════════════════════════╝
//
// HOW TO ADD A NEW EVENT:
//  1. Add constant to pkg/events/{domain}.go
//  2. Create event struct in use_cases/{domain}/domain/event/
//  3. Create handler in use_cases/{domain}/event/on_{event_name}_handler.go
//  4. Add eventBus.Subscribe() call below
//  5. Update docs/events/event_flows.puml
//  6. Run: make check-events
//
// Transport tiers (set in event_bus.go criticalTopics map):
//  Critical (SQL, durable):     orders.*, loyalty.*
//  Non-critical (go-channel):   notification.*, analytics.*

import (
	"context"

	core "goodin/internal"
	"goodin/pkg/cqrs"
	pkgevents "goodin/pkg/events"
	"goodin/repositories"
	loyaltycommand "goodin/use_cases/loyalty/command"
	"goodin/use_cases/loyalty/dto"
	loyaltyevent "goodin/use_cases/loyalty/event"
	loyaltyquery "goodin/use_cases/loyalty/query"

	"gorm.io/gorm"
)

// registerHandlers wires all domain handlers to the buses.
// This is the single authoritative source for command/query/event handler wiring
// in the non-FX path (cmd/all, cmd/http).
func registerHandlers(d *WatermillDriver) {
	app := d.app
	db := app.Data().Get("sql", "gorm").(*gorm.DB)

	registerLoyalty(app, db, d)
}

// registerLoyalty wires the entire loyalty domain into the buses.
// All repos and handlers are constructed here — callers only pass app, db, and the driver.
func registerLoyalty(app core.App, db *gorm.DB, d *WatermillDriver) {
	// ── Repositories ─────────────────────────────────────────────────────────
	writeRepo := repositories.NewLoyaltyWriteGormRepository(db)
	readRepo := repositories.NewLoyaltyReadGormRepository(db)
	idemRepo := repositories.NewIdempotencyGormRepository(db)
	cacheRepo := repositories.NewNoopLoyaltyCacheRepository()

	// ── Handlers ─────────────────────────────────────────────────────────────
	creditHandler := loyaltycommand.NewCreditLoyaltyHandler(app, writeRepo, idemRepo, d.eventBus)
	balanceHandler := loyaltyquery.NewGetLoyaltyBalanceHandler(app, readRepo, cacheRepo)
	onOrderHandler := loyaltyevent.NewOnOrderCompletedHandler(app, d.commandBus, d.eventBus)

	// ── Command bus ───────────────────────────────────────────────────────────
	d.commandBus.Register("loyalty.credit_loyalty", func(ctx context.Context, cmd cqrs.Command) (any, error) {
		return creditHandler.Handle(ctx, cmd.(dto.CreditLoyaltyRequest))
	})

	// ── Query bus ─────────────────────────────────────────────────────────────
	d.queryBus.Register("loyalty.get_balance", func(ctx context.Context, q cqrs.Query) (any, error) {
		return balanceHandler.Handle(ctx, q.(dto.GetLoyaltyBalanceRequest))
	})

	// ── Event bus ─────────────────────────────────────────────────────────────
	d.eventBus.Subscribe(pkgevents.OrderCompleted, onOrderHandler)
}
