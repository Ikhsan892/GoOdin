package saga

import (
	"context"
	"log/slog"
	"time"

	"goodin/pkg/cqrs"
	"goodin/pkg/events"

	core "goodin/internal"

	"github.com/oklog/ulid/v2"
)

// FulfillmentSagaState tracks whether both prerequisite events have been received
// for a given CorrelationID. When both are received, it publishes orders.fulfillment.ready.
//
// Persisted in DB (saga_states table) so progress survives app restarts.
type FulfillmentSagaState struct {
	CorrelationID     string
	LoyaltyCredited   bool
	InventoryReserved bool
	CompletedAt       *time.Time
}

// FulfillmentSagaStateRepository persists and retrieves saga state.
type FulfillmentSagaStateRepository interface {
	FindOrCreate(ctx context.Context, correlationID string) (*FulfillmentSagaState, error)
	Save(ctx context.Context, state *FulfillmentSagaState) error
}

// OrderFulfillmentSaga coordinates the order fulfillment flow.
// It subscribes to BOTH loyalty.account.credited AND inventory.stock.reserved.
// When both are received for the same CorrelationID, it publishes orders.fulfillment.ready.
//
// Registered in drivers/watermill/register.go for both event topics.
// This is the pattern for "Subscriber 3 waits for Subscriber 1 AND Subscriber 2".
type OrderFulfillmentSaga struct {
	app      core.App
	sagaRepo FulfillmentSagaStateRepository
	eventBus cqrs.EventBus
}

func NewOrderFulfillmentSaga(
	app core.App,
	sagaRepo FulfillmentSagaStateRepository,
	eventBus cqrs.EventBus,
) *OrderFulfillmentSaga {
	return &OrderFulfillmentSaga{
		app:      app,
		sagaRepo: sagaRepo,
		eventBus: eventBus,
	}
}

// Handle processes both loyalty.account.credited and inventory.stock.reserved events.
// It updates the saga state and publishes orders.fulfillment.ready when both are done.
func (s *OrderFulfillmentSaga) Handle(ctx context.Context, env cqrs.EventEnvelope) error {
	state, err := s.sagaRepo.FindOrCreate(ctx, env.CorrelationID)
	if err != nil {
		return err
	}

	// Already completed — idempotent
	if state.CompletedAt != nil {
		return nil
	}

	switch env.EventName {
	case events.LoyaltyCredited:
		state.LoyaltyCredited = true
	case "inventory.stock.reserved": // add constant to pkg/events/inventory.go when inventory domain exists
		state.InventoryReserved = true
	}

	if err := s.sagaRepo.Save(ctx, state); err != nil {
		return err
	}

	// Both prerequisites met — publish final event
	if state.LoyaltyCredited && state.InventoryReserved {
		now := time.Now()
		state.CompletedAt = &now
		if err := s.sagaRepo.Save(ctx, state); err != nil {
			return err
		}

		s.app.Logger().Info("order fulfillment saga: all prerequisites met — publishing fulfillment.ready",
			slog.String("correlation_id", env.CorrelationID))

		return s.eventBus.Publish(ctx, cqrs.EventEnvelope{
			EventID:       ulid.Make().String(),
			CorrelationID: env.CorrelationID,
			CausationID:   env.EventID,
			EventName:     events.OrderFulfillmentReady,
		})
	}

	return nil
}
