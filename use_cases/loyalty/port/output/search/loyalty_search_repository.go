package search

import "context"

// LoyaltySearchDocument is the MeiliSearch index document for a loyalty account.
type LoyaltySearchDocument struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Balance    int    `json:"balance"`
	Tier       string `json:"tier"`
	UpdatedAt  int64  `json:"updated_at"`
}

// LoyaltySearchRepository is the output port for MeiliSearch indexing and search.
// Implementation: repositories/loyalty/loyalty_search_meili_repository.go
type LoyaltySearchRepository interface {
	IndexAccount(ctx context.Context, doc LoyaltySearchDocument) error
	Search(ctx context.Context, query string) ([]LoyaltySearchDocument, error)
}
