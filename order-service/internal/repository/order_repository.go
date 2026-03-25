package repository

import (
	"context"
	"database/sql"

	"github.com/fahri-ris/dops-be.git/order-service/internal/domain"
)

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	query := `
		INSERT INTO orders (id, user_id, status, total, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		order.ID, order.UserID, order.Status, order.Total, order.CreatedAt, order.UpdatedAt,
	)
	return err
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	query := `
		SELECT id, user_id, status, total, created_at, updated_at
		FROM orders
		WHERE id = $1
	`
	order := &domain.Order{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&order.ID, &order.UserID, &order.Status, &order.Total, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (r *OrderRepository) Update(ctx context.Context, order *domain.Order) error {
	query := `
		UPDATE orders SET status = $1, total = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query, order.Status, order.Total, order.UpdatedAt, order.ID)
	return err
}

func (r *OrderRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, order *domain.Order) error {
	query := `
		INSERT INTO orders (id, user_id, status, total, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := tx.ExecContext(ctx, query,
		order.ID, order.UserID, order.Status, order.Total, order.CreatedAt, order.UpdatedAt,
	)
	return err
}

func (r *OrderRepository) UpdateWithTx(ctx context.Context, tx *sql.Tx, order *domain.Order) error {
	query := `
		UPDATE orders SET status = $1, total = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := tx.ExecContext(ctx, query, order.Status, order.Total, order.UpdatedAt, order.ID)
	return err
}

func (r *OrderRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}
