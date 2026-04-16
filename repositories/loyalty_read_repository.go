package repositories

import (
	"context"
	"errors"

	"goodin/gorm/models"
	"goodin/use_cases/loyalty/dto"
	"goodin/use_cases/loyalty/exception"
	"goodin/use_cases/loyalty/port/output/repository"

	"gorm.io/gorm"
)

var _ repository.LoyaltyReadRepository = (*loyaltyReadGormRepository)(nil)

type loyaltyReadGormRepository struct {
	db *gorm.DB
}

func NewLoyaltyReadGormRepository(db *gorm.DB) repository.LoyaltyReadRepository {
	return &loyaltyReadGormRepository{db: db}
}

func (r *loyaltyReadGormRepository) GetBalanceByCustomerID(ctx context.Context, customerID string) (dto.GetLoyaltyBalanceResponse, error) {
	var model models.LoyaltyAccount
	err := r.db.WithContext(ctx).
		Where("customer_id = ?", customerID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.GetLoyaltyBalanceResponse{}, exception.ErrLoyaltyAccountNotFound
		}
		return dto.GetLoyaltyBalanceResponse{}, err
	}
	return dto.GetLoyaltyBalanceResponse{
		LoyaltyAccountID: model.ID,
		CustomerID:       model.CustomerID,
		Balance:          model.Balance,
		Tier:             model.Tier,
	}, nil
}
