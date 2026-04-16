package events

// Orders domain event names.
// Every constant here MUST appear in drivers/watermill/register.go.
// Run `make check-events` to verify. See docs/events/event_flows.puml for topology.

// OrderCompleted is published after an order has been successfully persisted.
//
// Published by:
//
//	use_cases/orders/command/create_order_handler.go
//
// Subscribed by:
//
//	use_cases/loyalty/event/on_order_completed_handler.go   (credit loyalty points)
//	use_cases/notification/event/on_order_completed_handler.go (send confirmation)
const OrderCompleted = "orders.order.completed"

// OrderFulfillmentReady is published by the order fulfillment saga when
// BOTH loyalty.account.credited AND inventory.stock.reserved are received
// for the same CorrelationID.
//
// Published by:
//
//	use_cases/orders/saga/order_fulfillment_saga.go
//
// Subscribed by:
//
//	use_cases/shipping/event/on_fulfillment_ready_handler.go
const OrderFulfillmentReady = "orders.fulfillment.ready"

// OrderFailed is a compensating event. Published when a critical downstream
// saga step fails and the order must be marked as failed.
//
// Published by:
//
//	use_cases/orders/event/on_loyalty_credit_failed_handler.go
const OrderFailed = "orders.order.failed"
