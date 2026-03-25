package domain

import "context"

type InventoryEvent struct {
	ID          string
	OrderID     string
	ProcessedAt string
}

type InventoryRepository interface {
	IsOrderProcessed(ctx context.Context, orderID string) (bool, error)
	RecordProcessed(ctx context.Context, orderID string) error
	UpdateStock(ctx context.Context, productID string, quantity int) error
	GetStock(ctx context.Context, productID string) (int, error)
}
