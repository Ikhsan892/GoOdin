package repositories

import (
	"context"
	"time"

	"goodin/gorm/models"
	"goodin/use_cases/loyalty/port/output/idempotency"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var _ idempotency.IdempotencyRepository = (*idempotencyGormRepository)(nil)

type idempotencyGormRepository struct {
	db *gorm.DB
}

func NewIdempotencyGormRepository(db *gorm.DB) idempotency.IdempotencyRepository {
	return &idempotencyGormRepository{db: db}
}

func (r *idempotencyGormRepository) Exists(ctx context.Context, causationEventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.IdempotencyKey{}).
		Where("causation_event_id = ?", causationEventID).
		Count(&count).Error
	return count > 0, err
}

func (r *idempotencyGormRepository) Mark(ctx context.Context, causationEventID string) error {
	return r.db.WithContext(ctx).Create(&models.IdempotencyKey{
		ID:               uuid.New().String(),
		CausationEventID: causationEventID,
		Domain:           "loyalty",
		Handler:          "credit_loyalty",
		ProcessedAt:      time.Now(),
	}).Error
}
