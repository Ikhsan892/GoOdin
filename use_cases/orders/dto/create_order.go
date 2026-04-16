package dto

import validation "github.com/go-ozzo/ozzo-validation"

type CreateOrderRequest struct {
	CustomerName string             `json:"customer_name"`
	TotalAmount  float64            `json:"total_amount"`
	Address      string             `json:"address"`
	Items        []OrderItemRequest `json:"items"`
}

func (payload CreateOrderRequest) Validate() error {
	return validation.ValidateStruct(
		&payload,
		validation.Field(&payload.CustomerName, validation.Required),
		validation.Field(&payload.Address),
		validation.Field(&payload.TotalAmount, validation.Min(0)),
		validation.Field(&payload.Items),
	)
}

type OrderItemRequest struct {
	ProductId   string
	ProductName string
	Quantity    int32
	Price       float64
}

func (o OrderItemRequest) Validate() error {
	return validation.ValidateStruct(
		&o,
		validation.Field(&o.Price, validation.Required),
		validation.Field(&o.ProductId, validation.Required),
		validation.Field(&o.ProductName, validation.Required),
	)
}

type CreateOrderResponse struct {
	Id           string
	CreatedAt    int64
	UpdatedAt    int64
	Status       string
	CustomerName string
	TotalAmount  float64
	Address      string
	Items        []OrderItemRequest
}

type CreateOrderRepositoryResult struct {
	Id           string
	CustomerName string
	TotalAmount  float64
	Address      string
	Items        []OrderItemRequest
	Status       string
	CreatedAt    int64
	UpdatedAt    int64
}

type GetOrderByIDResult struct {
	Id           string
	CustomerName string
	TotalAmount  float64
	Address      string
	Status       string
}
