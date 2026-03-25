package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/fahri-ris/dops-be.git/order-service/internal/domain"
	"github.com/redis/go-redis/v9"
)

type ProductRepository struct {
	db    *sql.DB
	redis *redis.Client
}

func NewProductRepository(db *sql.DB, redis *redis.Client) *ProductRepository {
	return &ProductRepository{db: db, redis: redis}
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	cacheKey := "product:" + id
	cached, err := r.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var product domain.Product
		if json.Unmarshal([]byte(cached), &product) == nil {
			return &product, nil
		}
	}

	product, err := r.getByIDFromDB(ctx, id)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(product); err == nil {
		r.redis.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return product, nil
}

func (r *ProductRepository) getByIDFromDB(ctx context.Context, id string) (*domain.Product, error) {
	query := `
		SELECT id, name, price, stock, created_at, updated_at
		FROM products
		WHERE id = $1
	`
	product := &domain.Product{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&product.ID, &product.Name, &product.Price, &product.Stock, &product.CreatedAt, &product.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return product, nil
}

func (r *ProductRepository) UpdateStock(ctx context.Context, id string, quantity int) error {
	query := `
		UPDATE products SET stock = stock - $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, quantity, time.Now(), id)
	if err != nil {
		return err
	}

	cacheKey := "product:" + id
	r.redis.Del(ctx, cacheKey)

	return nil
}
