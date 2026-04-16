package loyalty

import (
	"context"

	nats_messaging "goodin/drivers/messaging/nats"
	nats_infra "goodin/infrastructure/messaging/nats"
	core "goodin/internal"
	"goodin/pkg/cqrs"
	pkgevents "goodin/pkg/events"
	"goodin/repositories"
	"goodin/use_cases/loyalty/command"
	"goodin/use_cases/loyalty/dto"
	loyaltyevent "goodin/use_cases/loyalty/event"
	"goodin/use_cases/loyalty/port/input/message/request"
	"goodin/use_cases/loyalty/query"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

var _ cqrs.HandlerRegistrar = (*Registrar)(nil)

// Registrar is the composition root for the loyalty domain in the FX world.
// It owns all dependency construction — repos, handlers, wiring.
// FX only needs to provide App and *gorm.DB; everything else is built here.
type Registrar struct {
	app core.App
	db  *gorm.DB
}

// Params uses fx.In so FX injects App and the named gorm DB automatically.
type Params struct {
	fx.In

	App core.App
	DB  *gorm.DB `name:"gorm"`
}

func NewRegistrar(p Params) *Registrar {
	return &Registrar{app: p.App, db: p.DB}
}

// Register is called once at startup by watermillfx.New.
// All repos and handlers are constructed here, keeping cmd/ free of loyalty wiring.
func (r *Registrar) Register(cmd cqrs.RegisterableCommandBus, qry cqrs.RegisterableQueryBus, evt cqrs.EventBus) {
	// ── Repositories ─────────────────────────────────────────────────────────
	writeRepo := repositories.NewLoyaltyWriteGormRepository(r.db)
	readRepo := repositories.NewLoyaltyReadGormRepository(r.db)
	idemRepo := repositories.NewIdempotencyGormRepository(r.db)
	cacheRepo := repositories.NewNoopLoyaltyCacheRepository()

	// ── ACL: Loyalty → Orders via NATS request-reply ─────────────────────────
	// Requires cmd/all.go (or cmd/message_broker.go) to have started the NATS driver.
	// Any handler that needs order data takes request.OrderDetailRequest and calls
	// orderDetailReq.GetOrderDetail(ctx, dto.GetOrderDetailRequest{OrderID: ...}).
	var orderDetailReq request.OrderDetailRequest
	if natsDrv := r.app.Driver().Instance(nats_messaging.NATS_DRIVER); natsDrv != nil {
		if infra, ok := natsDrv.(nats_infra.NatsInfrastructure); ok {
			orderDetailReq = NewOrderDetailRequestImpl(r.app, infra)
			r.app.Logger().Info("> Loyalty ACL wired: orders.get.detail via NATS")
		}
	}
	if orderDetailReq == nil {
		r.app.Logger().Warn("Loyalty ACL unavailable: NATS driver not running — GetOrderDetail calls will fail")
	}
	_ = orderDetailReq // pass into handlers that need it, e.g. NewCreditLoyaltyHandler(..., orderDetailReq)

	// ── Handlers ─────────────────────────────────────────────────────────────
	creditHandler := command.NewCreditLoyaltyHandler(r.app, writeRepo, idemRepo, evt)
	balanceHandler := query.NewGetLoyaltyBalanceHandler(r.app, readRepo, cacheRepo)
	onOrderHandler := loyaltyevent.NewOnOrderCompletedHandler(r.app, cmd, evt)

	// ── Command bus ───────────────────────────────────────────────────────────
	cmd.RegisterHandler("loyalty.credit_loyalty", func(ctx context.Context, c cqrs.Command) (any, error) {
		return creditHandler.Handle(ctx, c.(dto.CreditLoyaltyRequest))
	})

	// ── Query bus ─────────────────────────────────────────────────────────────
	qry.RegisterHandler("loyalty.get_balance", func(ctx context.Context, q cqrs.Query) (any, error) {
		return balanceHandler.Handle(ctx, q.(dto.GetLoyaltyBalanceRequest))
	})

	// ── Event bus ─────────────────────────────────────────────────────────────
	evt.Subscribe(pkgevents.OrderCompleted, onOrderHandler)
}
