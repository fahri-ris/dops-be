package domain

import "context"

type Payment struct {
	ID            string
	OrderID       string
	UserID        string
	Amount        float64
	PaymentMethod string
	Status        string
	TransactionID string
}

const (
	PaymentStatusPending = "PENDING"
	PaymentStatusSuccess = "SUCCESS"
	PaymentStatusFailed = "FAILED"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	GetByID(ctx context.Context, id string) (*Payment, error)
	GetByOrderID(ctx context.Context, orderID string) (*Payment, error)
	Update(ctx context.Context, payment *Payment) error
}

type PaymentService interface {
	ProcessPayment(ctx context.Context, orderID, userID string, amount float64, paymentMethod string) (*Payment, error)
	GetPayment(ctx context.Context, id string) (*Payment, error)
}
