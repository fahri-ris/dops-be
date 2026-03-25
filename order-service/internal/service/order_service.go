package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fahri-ris/dops-be.git/order-service/internal/domain"
	"github.com/fahri-ris/dops-be.git/order-service/internal/middleware"
	"github.com/google/uuid"
)

const DefaultPaymentMethod = "card"

type OrderService struct {
	orderRepo       domain.OrderRepository
	orderItemRepo   domain.OrderItemRepository
	productRepo     domain.ProductRepository
	paymentClient   domain.PaymentServiceClient
	eventPublisher  domain.EventPublisher
	logger          *slog.Logger
}

func NewOrderService(
	orderRepo domain.OrderRepository,
	orderItemRepo domain.OrderItemRepository,
	productRepo domain.ProductRepository,
	paymentClient domain.PaymentServiceClient,
	eventPublisher domain.EventPublisher,
	logger *slog.Logger,
) *OrderService {
	return &OrderService{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		productRepo:   productRepo,
		paymentClient: paymentClient,
		eventPublisher: eventPublisher,
		logger:        logger,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, userID string, input domain.CreateOrderInput) (*domain.Order, error) {
	traceID := middleware.GetTraceIDContext(ctx)
	logger := s.logger.With("trace_id", traceID, "user_id", userID)

	logger.Info("Creating order - starting transaction")

	// Validate input using go-playground/validator
	if err := domain.ValidateCreateOrderInput(input); err != nil {
		logger.Warn("Invalid order input", "error", err)
		return nil, fmt.Errorf("invalid order input: %w", err)
	}

	// Calculate total from cached product prices (cache-aside pattern)
	var total float64
	for _, item := range input.Items {
		product, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			logger.Error("Failed to get product", "product_id", item.ProductID, "error", err)
			return nil, fmt.Errorf("failed to get product %s: %w", item.ProductID, err)
		}
		total += product.Price * float64(item.Quantity)
	}

	now := time.Now()
	order := &domain.Order{
		ID:        uuid.New().String(),
		UserID:    userID,
		Status:    domain.OrderStatusPending,
		Total:     total,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Begin database transaction
	tx, err := s.orderRepo.BeginTx(ctx)
	if err != nil {
		logger.Error("Failed to begin transaction", "error", err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Flag to track if we need to rollback
	committed := false

	// Helper function to rollback on failure
	rollback := func() {
		if !committed {
			logger.Info("Rolling back transaction", "order_id", order.ID)
		}
	}

	// Create order within transaction
	if err := s.orderRepo.CreateWithTx(ctx, tx, order); err != nil {
		logger.Error("Failed to create order", "error", err)
		rollback()
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// Create order items within transaction
	for _, item := range input.Items {
		product, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			logger.Error("Failed to get product for item", "product_id", item.ProductID, "error", err)
			rollback()
			return nil, fmt.Errorf("failed to get product %s: %w", item.ProductID, err)
		}

		orderItem := &domain.OrderItem{
			ID:        uuid.New().String(),
			OrderID:   order.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     product.Price,
		}
		if err := s.orderItemRepo.CreateWithTx(ctx, tx, orderItem); err != nil {
			logger.Error("Failed to create order item", "error", err)
			rollback()
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}
		order.Items = append(order.Items, *orderItem)
	}

	// Call Payment Service via HTTP (mTLS)
	logger.Info("Calling payment service", "order_id", order.ID, "amount", total)

	paymentReq := domain.PaymentRequest{
		OrderID:        order.ID,
		UserID:         userID,
		Amount:         total,
		PaymentMethod:  DefaultPaymentMethod,
	}

	paymentResp, err := s.paymentClient.ProcessPayment(ctx, paymentReq)
	if err != nil {
		// Payment failed - rollback transaction and return 402
		logger.Warn("Payment failed - rolling back transaction",
			"order_id", order.ID,
			"error", err)
		rollback()
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	// Payment successful - update order status to PAID
	order.Status = domain.OrderStatusPaid
	order.UpdatedAt = time.Now()

	if err := s.orderRepo.UpdateWithTx(ctx, tx, order); err != nil {
		logger.Error("Failed to update order status", "error", err)
		rollback()
		return nil, fmt.Errorf("failed to update order status: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		logger.Error("Failed to commit transaction", "error", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	// Publish order.created event to NATS/Kafka
	if err := s.eventPublisher.PublishOrderCreated(ctx, order); err != nil {
		// Log error but don't fail the request - event publishing failure
		// should not rollback the committed transaction
		logger.Error("Failed to publish order.created event",
			"order_id", order.ID,
			"error", err)
	} else {
		logger.Info("Published order.created event", "order_id", order.ID)
	}

	logger.Info("Order created successfully",
		"order_id", order.ID,
		"status", order.Status,
		"payment_id", paymentResp.PaymentID)

	return order, nil
}

func (s *OrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	traceID := middleware.GetTraceIDContext(ctx)
	logger := s.logger.With("trace_id", traceID, "order_id", id)

	logger.Info("Getting order")

	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		logger.Error("Failed to get order", "error", err)
		return nil, err
	}

	items, err := s.orderItemRepo.GetByOrderID(ctx, id)
	if err != nil {
		logger.Error("Failed to get order items", "error", err)
		return nil, err
	}
	order.Items = items

	return order, nil
}
