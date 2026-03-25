package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fahri-ris/dops-be.git/order-service/internal/domain"
	"github.com/fahri-ris/dops-be.git/order-service/internal/middleware"
)

type OrderHandler struct {
	orderService domain.OrderService
	logger       *slog.Logger
}

func NewOrderHandler(orderService domain.OrderService, logger *slog.Logger) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		logger:       logger,
	}
}

type CreateOrderRequest struct {
	Items []struct {
		ProductID string  `json:"product_id"`
		Quantity  int     `json:"quantity"`
		Price     float64 `json:"price"`
	} `json:"items"`
}

type CreateOrderResponse struct {
	OrderID   string  `json:"order_id"`
	Status    string  `json:"status"`
	Total     float64 `json:"total"`
	CreatedAt string  `json:"created_at"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.GetTraceIDContext(r.Context())
	userID := r.Context().Value("user_id").(string)

	h.logger.Info("CreateOrder request received", "trace_id", traceID, "user_id", userID)

	// Decode request body
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "trace_id", traceID, "error", err)
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate request
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Order must have at least one item")
		return
	}

	// Call service
	input := domain.CreateOrderInput{
		Items: make([]domain.OrderItemInput, len(req.Items)),
	}
	for i, item := range req.Items {
		input.Items[i] = domain.OrderItemInput{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     item.Price,
		}
	}

	order, err := h.orderService.CreateOrder(r.Context(), userID, input)
	if err != nil {
		h.logger.Error("Failed to create order", "trace_id", traceID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create order")
		return
	}

	resp := CreateOrderResponse{
		OrderID:   order.ID,
		Status:    order.Status,
		Total:     order.Total,
		CreatedAt: order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	writeJSON(w, http.StatusCreated, resp)
}

type GetOrderResponse struct {
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	Status    string  `json:"status"`
	Total     float64 `json:"total"`
	Items     []struct {
		ProductID string  `json:"product_id"`
		Quantity  int     `json:"quantity"`
		Price     float64 `json:"price"`
	} `json:"items"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.GetTraceIDContext(r.Context())
	userID := r.Context().Value("user_id").(string)

	path := r.URL.Path
	orderID := path[strings.LastIndex(path, "/")+1:]

	h.logger.Info("GetOrder request received", "trace_id", traceID, "user_id", userID, "order_id", orderID)

	if orderID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Order ID is required")
		return
	}

	order, err := h.orderService.GetOrder(r.Context(), orderID)
	if err != nil {
		h.logger.Error("Failed to get order", "trace_id", traceID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get order")
		return
	}

	// Build response
	items := make([]struct {
		ProductID string  `json:"product_id"`
		Quantity  int     `json:"quantity"`
		Price     float64 `json:"price"`
	}, len(order.Items))
	for i, item := range order.Items {
		items[i] = struct {
			ProductID string  `json:"product_id"`
			Quantity  int     `json:"quantity"`
			Price     float64 `json:"price"`
		}{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     item.Price,
		}
	}

	resp := GetOrderResponse{
		OrderID:   order.ID,
		UserID:    order.UserID,
		Status:    order.Status,
		Total:     order.Total,
		Items:     items,
		CreatedAt: order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, errCode, message string) {
	writeJSON(w, status, ErrorResponse{Error: errCode, Message: message})
}
