package repositories

import (
	"context"
	"errors"

	"goodin/gorm/models"
	"goodin/use_cases/loyalty/domain/aggregate"
	"goodin/use_cases/loyalty/domain/entity"
	valueobject "goodin/use_cases/loyalty/domain/value_object"
	"goodin/use_cases/loyalty/port/output/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ repository.LoyaltyWriteRepository = (*loyaltyWriteGormRepository)(nil)

type loyaltyWriteGormRepository struct {
	db *gorm.DB
}

func NewLoyaltyWriteGormRepository(db *gorm.DB) repository.LoyaltyWriteRepository {
	return &loyaltyWriteGormRepository{db: db}
}

func (r *loyaltyWriteGormRepository) Save(ctx context.Context, agg *aggregate.LoyaltyAggregate) error {
	acc := agg.Account()
	model := models.LoyaltyAccount{
		ID:         acc.ID,
		CustomerID: acc.CustomerID,
		Balance:    acc.Balance.Value(),
		Tier:       string(acc.Tier),
		CreatedAt:  acc.CreatedAt,
		UpdatedAt:  acc.UpdatedAt,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"balance", "tier", "updated_at"}),
		}).
		Create(&model).Error
}

func (r *loyaltyWriteGormRepository) FindByCustomerID(ctx context.Context, customerID string) (*aggregate.LoyaltyAggregate, error) {
	var model models.LoyaltyAccount
	err := r.db.WithContext(ctx).
		Where("customer_id = ?", customerID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // caller creates a new aggregate
		}
		return nil, err
	}

	balance, err := valueobject.NewPoints(model.Balance)
	if err != nil {
		return nil, err
	}
	acc := entity.LoyaltyAccount{
		ID:         model.ID,
		CustomerID: model.CustomerID,
		Balance:    balance,
		Tier:       valueobject.Tier(model.Tier),
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}
	return aggregate.Reconstitute(acc), nil
}
