package watermillfx

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"goodin/pkg/cqrs"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var _ cqrs.EventBus = (*eventBus)(nil)

// criticalTopics lists event names that must use the durable SQL publisher.
// All other topics use the in-memory go-channel publisher.
var criticalTopics = map[string]bool{
	"orders.order.completed":        true,
	"loyalty.account.credited":      true,
	"loyalty.account.credit_failed": true,
	"orders.fulfillment.ready":      true,
	"orders.order.failed":           true,
}

type eventBus struct {
	router    *message.Router
	logger    watermill.LoggerAdapter
	gcChannel *gochannel.GoChannel

	// sqlPublisher and sqlSubscriberFn are set via SetSQLBackend after DB is ready.
	sqlPublisher    message.Publisher
	sqlSubscriberFn func(topic string) (message.Subscriber, error)

	subscribersMu sync.RWMutex
	subscribers   map[string][]cqrs.EventHandler
}

func newEventBus(router *message.Router, logger watermill.LoggerAdapter) *eventBus {
	gc := gochannel.NewGoChannel(gochannel.Config{
		OutputChannelBuffer: 256,
		Persistent:          false,
	}, logger)

	return &eventBus{
		router:      router,
		logger:      logger,
		gcChannel:   gc,
		subscribers: make(map[string][]cqrs.EventHandler),
	}
}

// SetSQLBackend injects a SQL publisher and subscriber factory for durable topics.
func (b *eventBus) SetSQLBackend(pub message.Publisher, subFn func(topic string) (message.Subscriber, error)) {
	b.sqlPublisher = pub
	b.sqlSubscriberFn = subFn
}

func (b *eventBus) Publish(ctx context.Context, envelopes ...cqrs.EventEnvelope) error {
	for _, env := range envelopes {
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
			pub = b.gcChannel
		}

		if err := pub.Publish(env.EventName, msg); err != nil {
			return fmt.Errorf("eventBus.Publish: topic=%s: %w", env.EventName, err)
		}
	}
	return nil
}

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

	b.router.AddConsumerHandler(
		handlerName,
		eventName,
		sub,
		func(msg *message.Message) error {
			var env cqrs.EventEnvelope
			if err := json.Unmarshal(msg.Payload, &env); err != nil {
				b.logger.Error("eventBus: failed to unmarshal envelope", err, watermill.LogFields{
					"topic": eventName,
				})
				msg.Ack()
				return nil
			}

			if err := handler.Handle(msg.Context(), env); err != nil {
				return err
			}
			return nil
		},
	)

	return nil
}
