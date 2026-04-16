package aggregate

import (
	"errors"
	"time"

	"goodin/pkg/cqrs"
	"goodin/use_cases/loyalty/domain/entity"
	"goodin/use_cases/loyalty/domain/event"
	valueobject "goodin/use_cases/loyalty/domain/value_object"

	"github.com/oklog/ulid/v2"
)

var ErrInsufficientPoints = errors.New("insufficient loyalty points")

// LoyaltyAggregate is the root aggregate for loyalty. It owns the LoyaltyAccount entity
// and enforces all business invariants. Domain events are collected internally and
// flushed after a successful repository save.
//
// Rule: never mutate the aggregate outside of its methods.
// Rule: never publish events before calling FlushEvents after a successful save.
type LoyaltyAggregate struct {
	account      entity.LoyaltyAccount
	domainEvents []cqrs.DomainEvent
}

// NewLoyaltyAggregate creates a fresh aggregate for a new customer.
func NewLoyaltyAggregate(customerID string) *LoyaltyAggregate {
	return &LoyaltyAggregate{
		account: entity.LoyaltyAccount{
			ID:         ulid.Make().String(),
			CustomerID: customerID,
			Balance:    valueobject.MustNewPoints(0),
			Tier:       valueobject.TierBronze,
			CreatedAt:  time.Now().Unix(),
			UpdatedAt:  time.Now().Unix(),
		},
	}
}

// Reconstitute rebuilds an aggregate from a persisted entity (used by write repository).
func Reconstitute(account entity.LoyaltyAccount) *LoyaltyAggregate {
	return &LoyaltyAggregate{account: account}
}

// CreditPoints adds points to the account and raises a LoyaltyCreditedEvent.
func (a *LoyaltyAggregate) CreditPoints(orderID string, points valueobject.Points) error {
	if points.IsZero() {
		return errors.New("cannot credit zero points")
	}

	a.account.Balance = a.account.Balance.Add(points)
	a.account.Tier = valueobject.CalculateTier(a.account.Balance)
	a.account.UpdatedAt = time.Now().Unix()

	a.domainEvents = append(a.domainEvents, event.NewLoyaltyCreditedEvent(
		a.account.ID,
		a.account.CustomerID,
		orderID,
		points.Value(),
		a.account.Balance.Value(),
		string(a.account.Tier),
	))

	return nil
}

// DebitPoints removes points from the account.
func (a *LoyaltyAggregate) DebitPoints(reason string, points valueobject.Points) error {
	newBalance, err := a.account.Balance.Sub(points)
	if err != nil {
		return ErrInsufficientPoints
	}

	a.account.Balance = newBalance
	a.account.Tier = valueobject.CalculateTier(a.account.Balance)
	a.account.UpdatedAt = time.Now().Unix()

	return nil
}

// Account returns a read-only copy of the underlying entity.
func (a *LoyaltyAggregate) Account() entity.LoyaltyAccount { return a.account }

// Balance returns the current point balance.
func (a *LoyaltyAggregate) Balance() int { return a.account.Balance.Value() }

// Tier returns the current tier.
func (a *LoyaltyAggregate) Tier() valueobject.Tier { return a.account.Tier }

// FlushEvents returns all collected domain events and clears the internal slice.
// Call this AFTER a successful repository save, then publish the events.
func (a *LoyaltyAggregate) FlushEvents() []cqrs.DomainEvent {
	evts := a.domainEvents
	a.domainEvents = nil
	return evts
}
