package domain

import (
	"context"
	"time"
)

type Order struct {
	ID        string
	UserID    string
	Status    string
	Total     float64
	Items     []OrderItem
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OrderItem struct {
	ID        string
	OrderID   string
	ProductID string
	Quantity  int
	Price     float64
}

const (
	OrderStatusPending   = "PENDING"
	OrderStatusPaid      = "PAID"
	OrderStatusFailed    = "FAILED"
	OrderStatusCompleted = "COMPLETED"
)

type CreateOrderInput struct {
	Items []struct {
		ProductID string
		Quantity  int
		Price     float64
	}
}

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id string) (*Order, error)
	Update(ctx context.Context, order *Order) error
}

type OrderService interface {
	CreateOrder(ctx context.Context, userID string, input CreateOrderInput) (*Order, error)
	GetOrder(ctx context.Context, id string) (*Order, error)
}

type OrderItemRepository interface {
	Create(ctx context.Context, item *OrderItem) error
	GetByOrderID(ctx context.Context, orderID string) ([]OrderItem, error)
}
