package request

import (
	"context"

	"goodin/use_cases/loyalty/dto"
)

type OrderDetailRequest interface {
	GetOrderDetail(ctx context.Context, req dto.GetOrderDetailRequest) (dto.OrderDetail, error)
}
