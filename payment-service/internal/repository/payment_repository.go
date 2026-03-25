package repository

import (
	"context"
	"database/sql"

	"github.com/fahri-ris/dops-be.git/payment-service/internal/domain"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	query := `
		INSERT INTO payments (id, order_id, user_id, amount, payment_method, status, transaction_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		payment.ID, payment.OrderID, payment.UserID, payment.Amount,
		payment.PaymentMethod, payment.Status, payment.TransactionID,
	)
	return err
}

func (r *PaymentRepository) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	query := `
		SELECT id, order_id, user_id, amount, payment_method, status, transaction_id
		FROM payments
		WHERE id = $1
	`
	payment := &domain.Payment{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount,
		&payment.PaymentMethod, &payment.Status, &payment.TransactionID,
	)
	if err != nil {
		return nil, err
	}
	return payment, nil
}

func (r *PaymentRepository) GetByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	query := `
		SELECT id, order_id, user_id, amount, payment_method, status, transaction_id
		FROM payments
		WHERE order_id = $1
	`
	payment := &domain.Payment{}
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount,
		&payment.PaymentMethod, &payment.Status, &payment.TransactionID,
	)
	if err != nil {
		return nil, err
	}
	return payment, nil
}

func (r *PaymentRepository) Update(ctx context.Context, payment *domain.Payment) error {
	query := `
		UPDATE payments SET status = $1, transaction_id = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, payment.Status, payment.TransactionID, payment.ID)
	return err
}
