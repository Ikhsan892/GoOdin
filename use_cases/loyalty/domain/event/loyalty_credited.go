package event

import "time"

// LoyaltyCreditedEvent is raised when loyalty points are successfully credited.
//
// Published by:
//
//	use_cases/loyalty/command/credit_loyalty_handler.go
//
// Subscribed by:
//
//	use_cases/notification/event/on_loyalty_credited_handler.go
//	use_cases/orders/saga/order_fulfillment_saga.go
//
// Compensated by: LoyaltyCreditFailedEvent
type LoyaltyCreditedEvent struct {
	LoyaltyAccountID string
	CustomerID       string
	OrderID          string
	PointsEarned     int
	NewBalance       int
	NewTier          string
	occurredAt       time.Time
}

func NewLoyaltyCreditedEvent(accountID, customerID, orderID string, earned, newBalance int, tier string) LoyaltyCreditedEvent {
	return LoyaltyCreditedEvent{
		LoyaltyAccountID: accountID,
		CustomerID:       customerID,
		OrderID:          orderID,
		PointsEarned:     earned,
		NewBalance:       newBalance,
		NewTier:          tier,
		occurredAt:       time.Now(),
	}
}

func (e LoyaltyCreditedEvent) EventName() string    { return "loyalty.account.credited" }
func (e LoyaltyCreditedEvent) OccurredAt() time.Time { return e.occurredAt }
func (e LoyaltyCreditedEvent) AggregateID() string   { return e.LoyaltyAccountID }
