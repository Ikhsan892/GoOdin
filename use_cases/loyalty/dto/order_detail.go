package dto

type GetOrderDetailRequest struct {
	OrderID string
}

// OrderDetail is Loyalty's own view of an order — only the fields Loyalty
// actually needs. Deliberately decoupled from the Orders domain model so the
// two domains can evolve independently.
type OrderDetail struct {
	OrderID      string
	CustomerName string
	TotalAmount  float64
	Status       string
}
