package event

import (
	"context"
	"log/slog"
	"time"

	"goodin/pkg/cqrs"
	"goodin/pkg/events"
	"goodin/use_cases/loyalty/domain/event"
	"goodin/use_cases/loyalty/dto"
	portevent "goodin/use_cases/loyalty/port/input/event"

	core "goodin/internal"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"

	"github.com/oklog/ulid/v2"
)

var _ portevent.OnOrderCompletedHandler = (*OnOrderCompletedHandler)(nil)

// OrderCompletedPayload is the expected payload shape from the orders domain.
// Must match what orders/command/create_order_handler.go publishes.
type OrderCompletedPayload struct {
	OrderID     string
	CustomerID  string
	TotalAmount float64
}

func (o OrderCompletedPayload) EventName() string   { return events.OrderCompleted }
func (o OrderCompletedPayload) OccurredAt() time.Time { return time.Time{} }
func (o OrderCompletedPayload) AggregateID() string   { return o.OrderID }

// OnOrderCompletedHandler reacts to orders.order.completed events.
// It dispatches the CreditLoyalty command and publishes a compensating event on failure.
//
// This handler is idempotent via the command handler's idempotency check.
// Transport: Watermill SQL (durable — replayed on crash until acked).
type OnOrderCompletedHandler struct {
	app        core.App
	commandBus cqrs.CommandBus
	eventBus   cqrs.EventBus
}

func NewOnOrderCompletedHandler(
	app core.App,
	commandBus cqrs.CommandBus,
	eventBus cqrs.EventBus,
) *OnOrderCompletedHandler {
	return &OnOrderCompletedHandler{
		app:        app,
		commandBus: commandBus,
		eventBus:   eventBus,
	}
}

// Handle processes the orders.order.completed event envelope.
func (h *OnOrderCompletedHandler) Handle(ctx context.Context, env cqrs.EventEnvelope) error {
	// Restore OTel trace context from envelope so this handler appears as a child span.
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(env.OtelCarrier))

	tracer := otel.Tracer("loyalty")
	ctx, span := tracer.Start(ctx, "event.on_order_completed")
	defer span.End()
	span.SetAttributes(
		attribute.String("event.id", env.EventID),
		attribute.String("correlation.id", env.CorrelationID),
		attribute.String("causation.id", env.CausationID),
	)

	// Type-assert payload
	payload, ok := env.Payload.(OrderCompletedPayload)
	if !ok {
		h.app.Logger().Error("loyalty: unexpected payload type for orders.order.completed",
			slog.String("event_id", env.EventID))
		return nil // ack — don't retry malformed messages
	}

	// Calculate points: 10% of order total (business rule)
	points := int(payload.TotalAmount * 0.10)
	if points < 1 {
		points = 1
	}

	// Dispatch command — command handler handles TX + idempotency + event publish
	_, err := h.commandBus.Dispatch(ctx, dto.CreditLoyaltyRequest{
		CustomerID:       payload.CustomerID,
		OrderID:          payload.OrderID,
		Points:           points,
		CorrelationID:    env.CorrelationID,
		CausationEventID: env.EventID,
	})

	if err != nil {
		h.app.Logger().Error("loyalty: failed to credit points — publishing compensation",
			slog.String("order_id", payload.OrderID),
			slog.String("error", err.Error()))

		// Publish compensating event (fat event — carries all context the compensator needs)
		compErr := h.eventBus.Publish(ctx, cqrs.EventEnvelope{
			EventID:       ulid.Make().String(),
			CorrelationID: env.CorrelationID,
			CausationID:   env.EventID,
			EventName:     events.LoyaltyCreditFailed,
			Payload: event.NewLoyaltyCreditFailedEvent(
				payload.CustomerID,
				payload.OrderID,
				err.Error(),
			),
		})
		if compErr != nil {
			h.app.Logger().Error("loyalty: failed to publish compensation event",
				slog.String("error", compErr.Error()))
		}

		return nil // ack — compensation is in flight; don't cause infinite retry loop
	}

	return nil
}
