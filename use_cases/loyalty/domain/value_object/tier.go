package valueobject

// Tier represents the loyalty tier of a customer account.
type Tier string

const (
	TierBronze Tier = "BRONZE"
	TierSilver Tier = "SILVER"
	TierGold   Tier = "GOLD"
)

// TierThresholds defines the minimum balance required to reach each tier.
var TierThresholds = map[Tier]int{
	TierBronze: 0,
	TierSilver: 500,
	TierGold:   2000,
}

// CalculateTier returns the tier for the given point balance.
func CalculateTier(balance Points) Tier {
	switch {
	case balance.Value() >= TierThresholds[TierGold]:
		return TierGold
	case balance.Value() >= TierThresholds[TierSilver]:
		return TierSilver
	default:
		return TierBronze
	}
}
