package repositories

import (
	"context"

	"goodin/gorm/models"
	"goodin/use_cases/orders/dto"
	"goodin/use_cases/orders/port/output/repository"

	"gorm.io/gorm"
)

var _ repository.OrderRepository = (*OrderGormRepository)(nil)

type OrderGormRepository struct {
	db *gorm.DB
}

func NewOrderPostgreRepository(db *gorm.DB) *OrderGormRepository {
	return &OrderGormRepository{
		db: db,
	}
}

// SaveOrder implements repository.OrderRepository.
func (o *OrderGormRepository) SaveOrder(ctx context.Context, payload dto.CreateOrderRequest) (dto.CreateOrderRepositoryResult, error) {
	p := models.Order{
		CustomerName: payload.CustomerName,
		TotalAmount:  payload.TotalAmount,
		Address:      payload.Address,
		Status:       "PENDING",
	}

	var items []dto.OrderItemRequest

	q1 := o.db.WithContext(ctx).Model(&models.OrderItem{})
	for _, item := range payload.Items {
		items_payload := models.OrderItem{
			ProductId:   item.ProductId,
			ProductName: item.ProductName,
			Price:       item.Price,
		}
		if err := q1.Create(&items_payload).Error; err != nil {
			return dto.CreateOrderRepositoryResult{}, err
		}

		items = append(items, dto.OrderItemRequest{
			ProductId:   item.ProductId,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
		})
	}

	if err := o.db.WithContext(ctx).Model(&models.Order{}).Create(&p).Error; err != nil {
		return dto.CreateOrderRepositoryResult{}, err
	}

	return dto.CreateOrderRepositoryResult{
		Id:           p.ID,
		CustomerName: p.CustomerName,
		TotalAmount:  p.TotalAmount,
		Address:      p.Address,
		CreatedAt:    p.Created,
		UpdatedAt:    p.Updated,
		Items:        items,
	}, nil
}

// FindByID implements repository.OrderRepository.
func (o *OrderGormRepository) FindByID(ctx context.Context, orderID string) (dto.GetOrderByIDResult, error) {
	var m models.Order
	if err := o.db.WithContext(ctx).First(&m, "id = ?", orderID).Error; err != nil {
		return dto.GetOrderByIDResult{}, err
	}
	return dto.GetOrderByIDResult{
		Id:           m.ID,
		CustomerName: m.CustomerName,
		TotalAmount:  m.TotalAmount,
		Address:      m.Address,
		Status:       m.Status,
	}, nil
}
