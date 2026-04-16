package watermill

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"goodin/pkg/cqrs"

	core "goodin/internal"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var _ cqrs.EventBus = (*eventBus)(nil)

// criticalTopics lists event names that must use the durable SQL publisher.
// All other topics use the in-memory go-channel publisher.
// Add new critical topics here as new domains are added.
var criticalTopics = map[string]bool{
	"orders.order.completed":        true,
	"loyalty.account.credited":      true,
	"loyalty.account.credit_failed": true,
	"orders.fulfillment.ready":      true,
	"orders.order.failed":           true,
}

// eventBus implements cqrs.EventBus using Watermill.
// Critical topics → Watermill SQL PubSub (durable, survives crash).
// Non-critical topics → Watermill go-channel (fast, in-memory).
type eventBus struct {
	app         core.App
	router      *message.Router
	logger      watermill.LoggerAdapter
	gcPublisher message.Publisher // go-channel for non-critical
	gcChannel   *gochannel.GoChannel

	// sqlPublisher and sqlSubscriberFn are set via SetSQLBackend()
	// after the DB connection is available (called from register.go).
	sqlPublisher    message.Publisher
	sqlSubscriberFn func(topic string) (message.Subscriber, error)

	subscribersMu sync.RWMutex
	subscribers   map[string][]cqrs.EventHandler
}

func newEventBus(app core.App, router *message.Router, logger watermill.LoggerAdapter) *eventBus {
	gc := gochannel.NewGoChannel(gochannel.Config{
		OutputChannelBuffer: 256,
		Persistent:          false,
	}, logger)

	return &eventBus{
		app:         app,
		router:      router,
		logger:      logger,
		gcPublisher: gc,
		gcChannel:   gc,
		subscribers: make(map[string][]cqrs.EventHandler),
	}
}

// SetSQLBackend injects a SQL publisher and subscriber factory for durable topics.
// Call this from register.go after the DB is available.
func (b *eventBus) SetSQLBackend(pub message.Publisher, subFn func(topic string) (message.Subscriber, error)) {
	b.sqlPublisher = pub
	b.sqlSubscriberFn = subFn
}

// Publish serialises and enqueues the envelope(s).
// Critical topics → SQL publisher (writes to watermill_messages, transactional if called with a TX context).
// Non-critical topics → go-channel publisher.
func (b *eventBus) Publish(ctx context.Context, envelopes ...cqrs.EventEnvelope) error {
	for _, env := range envelopes {
		// Inject OTel trace context into the envelope before serialising
		carrier := make(propagation.MapCarrier)
		otel.GetTextMapPropagator().Inject(ctx, carrier)
		env.OtelCarrier = carrier

		payload, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("eventBus.Publish: marshal failed: %w", err)
		}

		msg := message.NewMessage(watermill.NewUUID(), payload)

		var pub message.Publisher
		if criticalTopics[env.EventName] && b.sqlPublisher != nil {
			pub = b.sqlPublisher
		} else {
			pub = b.gcPublisher
		}

		if err := pub.Publish(env.EventName, msg); err != nil {
			return fmt.Errorf("eventBus.Publish: topic=%s: %w", env.EventName, err)
		}
	}
	return nil
}

// Subscribe registers an EventHandler for the given topic and wires it into the router.
// Called once per subscription from drivers/watermill/register.go during Init.
func (b *eventBus) Subscribe(eventName string, handler cqrs.EventHandler) error {
	b.subscribersMu.Lock()
	b.subscribers[eventName] = append(b.subscribers[eventName], handler)
	b.subscribersMu.Unlock()

	var sub message.Subscriber
	if criticalTopics[eventName] && b.sqlSubscriberFn != nil {
		var err error
		sub, err = b.sqlSubscriberFn(eventName)
		if err != nil {
			return fmt.Errorf("eventBus.Subscribe: SQL subscriber: %w", err)
		}
	} else {
		sub = b.gcChannel
	}

	handlerName := fmt.Sprintf("event_handler.%s.%d", eventName, len(b.subscribers[eventName]))
	b.router.AddNoPublisherHandler(
		handlerName,
		eventName,
		sub,
		func(msg *message.Message) error {
			var env cqrs.EventEnvelope
			if err := json.Unmarshal(msg.Payload, &env); err != nil {
				b.logger.Error("eventBus: failed to unmarshal envelope", err, watermill.LogFields{
					"topic": eventName,
				})
				msg.Ack() // ack to avoid infinite retry on bad messages
				return nil
			}

			if err := handler.Handle(msg.Context(), env); err != nil {
				return err // Nack — Watermill will retry (SQL: unacked row stays in table)
			}

			return nil
		},
	)

	return nil
}
