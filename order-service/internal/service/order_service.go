package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/fahri-ris/dops-be.git/order-service/internal/domain"
	"github.com/fahri-ris/dops-be.git/order-service/internal/middleware"
	"github.com/google/uuid"
)

type OrderService struct {
	orderRepo     domain.OrderRepository
	orderItemRepo domain.OrderItemRepository
	productRepo   domain.ProductRepository
	logger        *slog.Logger
}

func NewOrderService(
	orderRepo domain.OrderRepository,
	orderItemRepo domain.OrderItemRepository,
	productRepo domain.ProductRepository,
	logger *slog.Logger,
) *OrderService {
	return &OrderService{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		productRepo:   productRepo,
		logger:        logger,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, userID string, input domain.CreateOrderInput) (*domain.Order, error) {
	traceID := middleware.GetTraceIDContext(ctx)
	logger := s.logger.With("trace_id", traceID, "user_id", userID)

	logger.Info("Creating order")

	var total float64
	for _, item := range input.Items {
		total += item.Price * float64(item.Quantity)
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

	if err := s.orderRepo.Create(ctx, order); err != nil {
		logger.Error("Failed to create order", "error", err)
		return nil, err
	}

	for _, item := range input.Items {
		orderItem := &domain.OrderItem{
			ID:        uuid.New().String(),
			OrderID:   order.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     item.Price,
		}
		if err := s.orderItemRepo.Create(ctx, orderItem); err != nil {
			logger.Error("Failed to create order item", "error", err)
			return nil, err
		}
		order.Items = append(order.Items, *orderItem)
	}

	logger.Info("Order created successfully", "order_id", order.ID)
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
