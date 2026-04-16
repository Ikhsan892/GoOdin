package models

import "time"

// IdempotencyKey is the GORM model for the idempotency_keys table.
// Shared across all domains — the Domain + Handler columns identify which handler processed the event.
// See migrations/000005_create_event_support_tables.up.sql.
type IdempotencyKey struct {
	ID               string    `gorm:"type:varchar(36);primaryKey"`
	CausationEventID string    `gorm:"type:varchar(36);uniqueIndex;column:causation_event_id"`
	Domain           string    `gorm:"type:varchar(100)"`
	Handler          string    `gorm:"type:varchar(100)"`
	ProcessedAt      time.Time `gorm:"column:processed_at"`
}

func (IdempotencyKey) TableName() string { return "idempotency_keys" }
