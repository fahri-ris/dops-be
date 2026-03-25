package domain

import (
	"context"
	"database/sql"
	"errors"
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
	Items []OrderItemInput
}

type OrderItemInput struct {
	ProductID string
	Quantity  int
	Price     float64
}

type PaymentRequest struct {
	OrderID        string  `json:"order_id"`
	UserID         string  `json:"user_id"`
	Amount         float64 `json:"amount"`
	PaymentMethod  string  `json:"payment_method"`
}

type PaymentResponse struct {
	PaymentID     string `json:"payment_id"`
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id"`
}

var (
	ErrInvalidOrderInput = errors.New("invalid order input")
	ErrEmptyItems        = errors.New("order must have at least one item")
	ErrInvalidProductID  = errors.New("product_id is required")
	ErrInvalidQuantity    = errors.New("quantity must be at least 1")
	ErrInvalidPrice      = errors.New("price must be greater than 0")
)

func ValidateCreateOrderInput(input CreateOrderInput) error {
	if len(input.Items) == 0 {
		return ErrEmptyItems
	}

	for i, item := range input.Items {
		if item.ProductID == "" {
			return errors.New("item product_id is required")
		}
		if item.Quantity < 1 {
			return errors.New("item quantity must be at least 1")
		}
		if item.Price <= 0 {
			return errors.New("item price must be greater than 0")
		}
		_ = i // unused but kept for potential future use
	}

	return nil
}

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	CreateWithTx(ctx context.Context, tx *sql.Tx, order *Order) error
	GetByID(ctx context.Context, id string) (*Order, error)
	Update(ctx context.Context, order *Order) error
	UpdateWithTx(ctx context.Context, tx *sql.Tx, order *Order) error
	BeginTx(ctx context.Context) (*sql.Tx, error)
}

type OrderService interface {
	CreateOrder(ctx context.Context, userID string, input CreateOrderInput) (*Order, error)
	GetOrder(ctx context.Context, id string) (*Order, error)
}

type OrderItemRepository interface {
	Create(ctx context.Context, item *OrderItem) error
	CreateWithTx(ctx context.Context, tx *sql.Tx, item *OrderItem) error
	GetByOrderID(ctx context.Context, orderID string) ([]OrderItem, error)
}

type PaymentServiceClient interface {
	ProcessPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
}

type EventPublisher interface {
	PublishOrderCreated(ctx context.Context, order *Order) error
}
