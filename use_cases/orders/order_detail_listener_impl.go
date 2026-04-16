package orders

import (
	"context"
	"encoding/json"
	"log/slog"

	nats_infra "goodin/infrastructure/messaging/nats"
	core "goodin/internal"
	"goodin/use_cases/orders/port/input/message/listener"
	"goodin/use_cases/orders/port/output/repository"

	"github.com/nats-io/nats.go"
)

var _ listener.OrderDetailListener = (*OrderDetailListenerImpl)(nil)

const subjectGetOrderDetail = "orders.get.detail"

type orderDetailRequestWire struct {
	OrderID string `json:"order_id"`
}

type orderDetailReplyWire struct {
	OrderID      string  `json:"order_id"`
	CustomerName string  `json:"customer_name"`
	TotalAmount  float64 `json:"total_amount"`
	Status       string  `json:"status"`
}

type OrderDetailListenerImpl struct {
	app   core.App
	infra nats_infra.NatsInfrastructure
	repo  repository.OrderRepository
	sub   *nats.Subscription
}

func NewOrderDetailListenerImpl(app core.App, infra nats_infra.NatsInfrastructure, repo repository.OrderRepository) *OrderDetailListenerImpl {
	return &OrderDetailListenerImpl{app: app, infra: infra, repo: repo}
}

func (l *OrderDetailListenerImpl) Start() error {
	sub, err := l.infra.Subscribe(subjectGetOrderDetail, func(msg *nats.Msg) {
		var req orderDetailRequestWire
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			l.app.Logger().Error("order_detail_listener: bad request",
				slog.String("err", err.Error()))
			return
		}

		order, err := l.repo.FindByID(context.Background(), req.OrderID)
		if err != nil {
			l.app.Logger().Error("order_detail_listener: find order failed",
				slog.String("order_id", req.OrderID),
				slog.String("err", err.Error()))
			return
		}

		reply, _ := json.Marshal(orderDetailReplyWire{
			OrderID:      order.Id,
			CustomerName: order.CustomerName,
			TotalAmount:  order.TotalAmount,
			Status:       order.Status,
		})
		if err := msg.Respond(reply); err != nil {
			l.app.Logger().Error("order_detail_listener: respond failed",
				slog.String("err", err.Error()))
		}
	})
	if err != nil {
		return err
	}
	l.sub = sub
	return nil
}

func (l *OrderDetailListenerImpl) Stop() error {
	if l.sub != nil {
		return l.sub.Unsubscribe()
	}
	return nil
}
