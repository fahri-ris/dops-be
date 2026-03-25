package domain

import (
	"context"
	"time"
)

type Product struct {
	ID        string
	Name      string
	Price     float64
	Stock     int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProductRepository interface {
	GetByID(ctx context.Context, id string) (*Product, error)
	UpdateStock(ctx context.Context, id string, quantity int) error
}
