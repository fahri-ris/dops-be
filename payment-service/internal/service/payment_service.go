package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/fahri-ris/dops-be.git/payment-service/internal/domain"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/middleware"
	"github.com/google/uuid"
)

type PaymentService struct {
	paymentRepo domain.PaymentRepository
	logger      *slog.Logger
}

func NewPaymentService(paymentRepo domain.PaymentRepository, logger *slog.Logger) *PaymentService {
	return &PaymentService{
		paymentRepo: paymentRepo,
		logger:      logger,
	}
}

func (s *PaymentService) ProcessPayment(ctx context.Context, orderID, userID string, amount float64, paymentMethod string) (*domain.Payment, error) {
	traceID := middleware.GetTraceIDContext(ctx)
	logger := s.logger.With("trace_id", traceID, "order_id", orderID)

	logger.Info("Processing payment", "amount", amount, "method", paymentMethod)

	existing, err := s.paymentRepo.GetByOrderID(ctx, orderID)
	if err == nil && existing != nil {
		logger.Info("Payment already exists for order", "payment_id", existing.ID)
		return existing, nil
	}

	payment := &domain.Payment{
		ID:            uuid.New().String(),
		OrderID:       orderID,
		UserID:        userID,
		Amount:        amount,
		PaymentMethod: paymentMethod,
		Status:        domain.PaymentStatusPending,
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		logger.Error("Failed to create payment record", "error", err)
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	payment.Status = domain.PaymentStatusSuccess
	payment.TransactionID = uuid.New().String()

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		logger.Error("Failed to update payment status", "error", err)
		return nil, err
	}

	logger.Info("Payment processed successfully", "payment_id", payment.ID, "transaction_id", payment.TransactionID)
	return payment, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id string) (*domain.Payment, error) {
	traceID := middleware.GetTraceIDContext(ctx)
	logger := s.logger.With("trace_id", traceID, "payment_id", id)

	logger.Info("Getting payment")
	return s.paymentRepo.GetByID(ctx, id)
}
