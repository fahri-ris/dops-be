package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/fahri-ris/dops-be.git/payment-service/internal/domain"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/middleware"
)

type PaymentHandler struct {
	paymentService domain.PaymentService
	logger         *slog.Logger
}

func NewPaymentHandler(paymentService domain.PaymentService, logger *slog.Logger) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		logger:         logger,
	}
}

type ProcessPaymentRequest struct {
	OrderID       string  `json:"order_id"`
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
}

type ProcessPaymentResponse struct {
	PaymentID     string `json:"payment_id"`
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (h *PaymentHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.GetTraceIDContext(r.Context())
	h.logger.Info("ProcessPayment request received", "trace_id", traceID)

	var req ProcessPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "trace_id", traceID, "error", err)
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.OrderID == "" || req.UserID == "" || req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Missing required fields")
		return
	}

	payment, err := h.paymentService.ProcessPayment(r.Context(), req.OrderID, req.UserID, req.Amount, req.PaymentMethod)
	if err != nil {
		h.logger.Error("Failed to process payment", "trace_id", traceID, "error", err)
		writeError(w, http.StatusBadRequest, "payment_failed", "Payment processing failed")
		return
	}

	resp := ProcessPaymentResponse{
		PaymentID:     payment.ID,
		Status:        payment.Status,
		TransactionID: payment.TransactionID,
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, error, message string) {
	writeJSON(w, status, ErrorResponse{Error: error, Message: message})
}
