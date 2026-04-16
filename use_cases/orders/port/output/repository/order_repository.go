package repository

import (
	"context"

	"mox/use_cases/orders/dto"
)

type OrderRepository interface {
	SaveOrder(ctx context.Context, payload dto.CreateOrderRequest) (dto.CreateOrderRepositoryResult, error) // dto.CreateOrderRepositoryResult can be changed to dto.CreateOrderResult if possible
	FindByID(ctx context.Context, orderID string) (dto.GetOrderByIDResult, error)
}
