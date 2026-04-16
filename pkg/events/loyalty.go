package events

// Loyalty domain event names.
// Every constant here MUST appear in drivers/watermill/register.go.
// Run `make check-events` to verify. See docs/events/event_flows.puml for topology.

// LoyaltyCredited is published after loyalty points have been successfully credited
// to a customer's account.
//
// Published by:
//
//	use_cases/loyalty/command/credit_loyalty_handler.go
//
// Subscribed by:
//
//	use_cases/notification/event/on_loyalty_credited_handler.go
//	use_cases/orders/saga/order_fulfillment_saga.go
const LoyaltyCredited = "loyalty.account.credited"

// LoyaltyCreditFailed is a compensating event published when loyalty crediting fails.
// Triggers the order compensation flow.
//
// Published by:
//
//	use_cases/loyalty/event/on_order_completed_handler.go
//
// Subscribed by:
//
//	use_cases/orders/event/on_loyalty_credit_failed_handler.go
const LoyaltyCreditFailed = "loyalty.account.credit_failed"
