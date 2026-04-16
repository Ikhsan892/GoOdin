package event

import "time"

// LoyaltyCreditFailedEvent is a compensating event raised when loyalty crediting fails.
// It carries all context needed by downstream compensators (fat event pattern).
//
// Published by:
//
//	use_cases/loyalty/event/on_order_completed_handler.go
//
// Subscribed by:
//
//	use_cases/orders/event/on_loyalty_credit_failed_handler.go
type LoyaltyCreditFailedEvent struct {
	// Fields from the original triggering event — carried forward so compensators
	// have everything they need without looking up other services.
	CustomerID string
	OrderID    string

	// Failure context
	Reason     string
	occurredAt time.Time
}

func NewLoyaltyCreditFailedEvent(customerID, orderID, reason string) LoyaltyCreditFailedEvent {
	return LoyaltyCreditFailedEvent{
		CustomerID: customerID,
		OrderID:    orderID,
		Reason:     reason,
		occurredAt: time.Now(),
	}
}

func (e LoyaltyCreditFailedEvent) EventName() string    { return "loyalty.account.credit_failed" }
func (e LoyaltyCreditFailedEvent) OccurredAt() time.Time { return e.occurredAt }
func (e LoyaltyCreditFailedEvent) AggregateID() string   { return e.CustomerID }
