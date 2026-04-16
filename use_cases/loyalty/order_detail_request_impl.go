package loyalty

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	nats_infra "goodin/infrastructure/messaging/nats"
	core "goodin/internal"
	"goodin/use_cases/loyalty/dto"
	"goodin/use_cases/loyalty/port/input/message/request"

	"github.com/nats-io/nats.go"
	"github.com/sony/gobreaker/v2"
)

var _ request.OrderDetailRequest = (*OrderDetailRequestImpl)(nil)

const (
	subjectGetOrderDetail = "orders.get.detail"
	requestTimeout        = 2 * time.Second
)

// Wire shapes are kept private to this file so Loyalty's public DTO never
// leaks the foreign (Orders) payload format.
type orderDetailRequestWire struct {
	OrderID string `json:"order_id"`
}

type orderDetailReplyWire struct {
	OrderID      string  `json:"order_id"`
	CustomerName string  `json:"customer_name"`
	TotalAmount  float64 `json:"total_amount"`
	Status       string  `json:"status"`
}

type OrderDetailRequestImpl struct {
	app   core.App
	infra nats_infra.NatsInfrastructure
	cb    *gobreaker.CircuitBreaker[*nats.Msg]
}

func NewOrderDetailRequestImpl(app core.App, infra nats_infra.NatsInfrastructure) *OrderDetailRequestImpl {
	var st gobreaker.Settings
	st.Name = "Loyalty->Orders OrderDetail"
	st.Timeout = 3 * time.Second
	st.MaxRequests = 3
	st.ReadyToTrip = func(c gobreaker.Counts) bool {
		if c.Requests < 3 {
			return false
		}
		return float64(c.TotalFailures)/float64(c.Requests) >= 0.6
	}
	st.OnStateChange = func(name string, from, to gobreaker.State) {
		app.Logger().Debug("circuit state change",
			slog.String("name", name),
			slog.String("from", from.String()),
			slog.String("to", to.String()))
	}

	return &OrderDetailRequestImpl{
		app:   app,
		infra: infra,
		cb:    gobreaker.NewCircuitBreaker[*nats.Msg](st),
	}
}

func (r *OrderDetailRequestImpl) GetOrderDetail(ctx context.Context, req dto.GetOrderDetailRequest) (dto.OrderDetail, error) {
	payload, err := json.Marshal(orderDetailRequestWire{OrderID: req.OrderID})
	if err != nil {
		return dto.OrderDetail{}, fmt.Errorf("marshal request: %w", err)
	}

	msg, err := r.cb.Execute(func() (*nats.Msg, error) {
		return r.infra.Request(subjectGetOrderDetail, payload, requestTimeout)
	})
	if err != nil {
		return dto.OrderDetail{}, fmt.Errorf("nats request: %w", err)
	}

	var wire orderDetailReplyWire
	if err := json.Unmarshal(msg.Data, &wire); err != nil {
		return dto.OrderDetail{}, fmt.Errorf("unmarshal reply: %w", err)
	}

	// Translation boundary: foreign wire shape -> Loyalty-native DTO.
	return dto.OrderDetail{
		OrderID:      wire.OrderID,
		CustomerName: wire.CustomerName,
		TotalAmount:  wire.TotalAmount,
		Status:       wire.Status,
	}, nil
}
