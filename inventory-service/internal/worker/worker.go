package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/fahri-ris/dops-be.git/inventory-service/internal/domain"
)

type OrderMessage struct {
	OrderID   string      `json:"order_id"`
	UserID    string      `json:"user_id"`
	Total     float64     `json:"total"`
	Items     []OrderItem `json:"items"`
	TraceID   string      `json:"trace_id"`
}

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Worker struct {
	repo       domain.InventoryRepository
	logger     *slog.Logger
	maxRetries int
}

func NewWorker(repo domain.InventoryRepository, logger *slog.Logger, maxRetries int) *Worker {
	return &Worker{
		repo:       repo,
		logger:     logger,
		maxRetries: maxRetries,
	}
}

func (w *Worker) Process(ctx context.Context, msg OrderMessage) error {
	logger := w.logger.With(
		"trace_id", msg.TraceID,
		"order_id", msg.OrderID,
	)

	logger.Info("Processing order")

	processed, err := w.repo.IsOrderProcessed(ctx, msg.OrderID)
	if err != nil {
		logger.Error("Failed to check idempotency", "error", err)
		return err
	}
	if processed {
		logger.Info("Order already processed, skipping")
		return nil
	}

	for _, item := range msg.Items {
		itemLogger := logger.With("product_id", item.ProductID, "quantity", item.Quantity)

		var lastErr error
		for attempt := 0; attempt < w.maxRetries; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				itemLogger.Info("Retrying after backoff", "attempt", attempt+1, "backoff", backoff)
				time.Sleep(backoff)
			}

			err := w.repo.UpdateStock(ctx, item.ProductID, item.Quantity)
			if err == nil {
				itemLogger.Info("Stock updated successfully")
				break
			}
			lastErr = err
			itemLogger.Error("Failed to update stock", "error", err, "attempt", attempt+1)
		}

		if lastErr != nil {
			itemLogger.Error("Failed to update stock after all retries", "error", lastErr)
			return lastErr
		}
	}

	if err := w.repo.RecordProcessed(ctx, msg.OrderID); err != nil {
		logger.Error("Failed to record processed order", "error", err)
		return err
	}

	logger.Info("Order processed successfully")
	return nil
}
