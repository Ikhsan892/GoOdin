package dto

// GetLoyaltyBalanceRequest is the query payload for fetching a customer's balance.
type GetLoyaltyBalanceRequest struct {
	CustomerID string
}

func (g GetLoyaltyBalanceRequest) QueryName() string { return "loyalty.get_balance" }

// GetLoyaltyBalanceResponse is the read model returned by GetLoyaltyBalanceHandler.
type GetLoyaltyBalanceResponse struct {
	LoyaltyAccountID string
	CustomerID       string
	Balance          int
	Tier             string
}
