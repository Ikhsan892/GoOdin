package nats_messaging

import (
	"log/slog"

	"goodin/drivers/messaging/nats/listener"
	"goodin/repositories"
	"goodin/use_cases/orders"

	"gorm.io/gorm"
)

func registerListener(n *NatsMesssaging) {
	order, err := listener.NewOrderListener(n.app, n.infra, listener.OrderListenerConfig{
		Stream: "orders",
	})
	if err != nil {
		n.app.Logger().Error("Error initialization order listener", slog.String("msg", err.Error()))
	}

	order.SubscribeEphemeral()

	// ACL responder: Loyalty (or any other domain) calls "orders.get.detail"
	// via NATS request-reply to fetch an order by ID. The listener translates
	// the wire format to/from the Orders repository.
	db, _ := n.app.Data().Get("sql", "gorm").(*gorm.DB)
	if db == nil {
		n.app.Logger().Warn("order_detail_listener: gorm DB unavailable, skipping ACL responder")
		return
	}
	orderRepo := repositories.NewOrderPostgreRepository(db)
	detailListener := orders.NewOrderDetailListenerImpl(n.app, n.infra, orderRepo)
	if err := detailListener.Start(); err != nil {
		n.app.Logger().Error("order_detail_listener: failed to start", slog.String("msg", err.Error()))
	}
}
