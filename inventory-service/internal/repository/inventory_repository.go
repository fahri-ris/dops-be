package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type InventoryRepository struct {
	db *sql.DB
}

func NewInventoryRepository(db *sql.DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

func (r *InventoryRepository) IsOrderProcessed(ctx context.Context, orderID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM inventory_events WHERE order_id = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(&exists)
	return exists, err
}

func (r *InventoryRepository) RecordProcessed(ctx context.Context, orderID string) error {
	query := `
		INSERT INTO inventory_events (id, order_id, processed_at)
		VALUES ($1, $2, $3)
	`
	_, err := r.db.ExecContext(ctx, query, uuid.New().String(), orderID, time.Now())
	return err
}

func (r *InventoryRepository) UpdateStock(ctx context.Context, productID string, quantity int) error {
	query := `
		UPDATE products SET stock = stock - $1, updated_at = $2
		WHERE id = $3 AND stock >= $1
	`
	result, err := r.db.ExecContext(ctx, query, quantity, time.Now(), productID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *InventoryRepository) GetStock(ctx context.Context, productID string) (int, error) {
	query := `SELECT stock FROM products WHERE id = $1`
	var stock int
	err := r.db.QueryRowContext(ctx, query, productID).Scan(&stock)
	return stock, err
}
