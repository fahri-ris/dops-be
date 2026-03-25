package repository

import (
	"context"
	"database/sql"

	"github.com/fahri-ris/dops-be.git/order-service/internal/domain"
)

type OrderItemRepository struct {
	db *sql.DB
}

func NewOrderItemRepository(db *sql.DB) *OrderItemRepository {
	return &OrderItemRepository{db: db}
}

func (r *OrderItemRepository) Create(ctx context.Context, item *domain.OrderItem) error {
	query := `
		INSERT INTO order_items (id, order_id, product_id, quantity, price)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query,
		item.ID, item.OrderID, item.ProductID, item.Quantity, item.Price,
	)
	return err
}

func (r *OrderItemRepository) GetByOrderID(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
	query := `
		SELECT id, order_id, product_id, quantity, price
		FROM order_items
		WHERE order_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Quantity, &item.Price); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *OrderItemRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, item *domain.OrderItem) error {
	query := `
		INSERT INTO order_items (id, order_id, product_id, quantity, price)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.ExecContext(ctx, query,
		item.ID, item.OrderID, item.ProductID, item.Quantity, item.Price,
	)
	return err
}
