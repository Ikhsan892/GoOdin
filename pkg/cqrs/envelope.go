package cqrs

import (
	"time"

	"go.opentelemetry.io/otel/propagation"
)

// EventEnvelope wraps a DomainEvent with routing and tracing metadata.
// Every event published via EventBus is wrapped in this envelope.
// The envelope is the unit of transport — never pass raw DomainEvent to EventBus.
type EventEnvelope struct {
	// EventID uniquely identifies this event instance (use ulid or uuid).
	EventID string

	// CorrelationID groups all events in a single business transaction chain.
	// Set once at the HTTP handler from the OTel trace ID; carried forward unchanged.
	// Filter logs by this field to see the complete event chain for one request.
	CorrelationID string

	// CausationID is the EventID (or CommandID) that directly caused this event.
	// Forms a causal chain: CausationID → EventID → next event's CausationID.
	CausationID string

	// EventName is the topic this event is published to.
	// Must match a constant defined in pkg/events/ — never use raw strings.
	EventName string

	// OccurredAt is the wall-clock time when the domain event was raised.
	OccurredAt time.Time

	// OtelCarrier carries the OpenTelemetry W3C trace context across the event bus.
	// Set by EventBus.Publish from the caller's context.
	// Extracted by each handler via otel.GetTextMapPropagator().Extract(ctx, env.OtelCarrier).
	OtelCarrier propagation.MapCarrier

	// Payload is the typed domain event. Type-assert to the concrete struct.
	// Example: payload := env.Payload.(loyaltyevent.LoyaltyCreditedEvent)
	Payload DomainEvent
}
