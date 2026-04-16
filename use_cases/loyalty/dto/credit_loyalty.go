package dto

// CreditLoyaltyRequest is the command payload for crediting loyalty points.
// Includes routing fields (CorrelationID, CausationEventID) for tracing and idempotency.
type CreditLoyaltyRequest struct {
	CustomerID      string
	OrderID         string
	Points          int
	CorrelationID   string
	CausationEventID string
}

func (c CreditLoyaltyRequest) CommandName() string { return "loyalty.credit_loyalty" }

// CreditLoyaltyResponse is returned after successful crediting.
type CreditLoyaltyResponse struct {
	LoyaltyAccountID string
	NewBalance       int
	NewTier          string
}
