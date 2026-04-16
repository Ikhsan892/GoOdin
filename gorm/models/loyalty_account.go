package models

// LoyaltyAccount is the GORM model for the loyalty_accounts table.
// Maps directly to the write-model schema (see migrations/000005_create_event_support_tables.up.sql).
// Do not add business logic here — all rules live in the domain aggregate.
type LoyaltyAccount struct {
	ID         string `gorm:"type:varchar(36);primaryKey"`
	CustomerID string `gorm:"type:varchar(36);uniqueIndex;column:customer_id"`
	Balance    int    `gorm:"not null;default:0"`
	Tier       string `gorm:"type:varchar(20);not null;default:'BRONZE'"`
	CreatedAt  int64  `gorm:"column:created_at;autoCreateTime:milli"`
	UpdatedAt  int64  `gorm:"column:updated_at;autoUpdateTime:milli"`
}

func (LoyaltyAccount) TableName() string { return "loyalty_accounts" }
