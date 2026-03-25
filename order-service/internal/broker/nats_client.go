package broker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/fahri-ris/dops-be.git/order-service/internal/domain"
	"github.com/fahri-ris/dops-be.git/order-service/internal/middleware"
	"github.com/nats-io/nats.go"
)

const (
	OrderCreatedSubject = "order.created"
)

type NATSClient struct {
	conn   *nats.Conn
	logger *slog.Logger
}

func NewNATSClient(url string, logger *slog.Logger) (*NATSClient, error) {
	conn, err := nats.Connect(url,
		nats.Name("order-service"),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, err
	}

	logger.Info("Connected to NATS", "url", url)

	return &NATSClient{
		conn:   conn,
		logger: logger,
	}, nil
}

func (c *NATSClient) PublishOrderCreated(ctx context.Context, order *domain.Order) error {
	traceID := middleware.GetTraceIDContext(ctx)

	event := OrderCreatedEvent{
		OrderID:   order.ID,
		UserID:    order.UserID,
		Status:    order.Status,
		Total:     order.Total,
		Items:     order.Items,
		CreatedAt: order.CreatedAt,
		TraceID:   traceID,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Create message with headers for trace context
	msg := &nats.Msg{
		Subject: OrderCreatedSubject,
		Data:    payload,
		Header: nats.Header{
			"trace_id":     []string{traceID},
			"traceparent":  []string{formatTraceparent(traceID)},
		},
	}

	if err := c.conn.PublishMsg(msg); err != nil {
		c.logger.Error("Failed to publish order.created event",
			"trace_id", traceID,
			"order_id", order.ID,
			"error", err)
		return err
	}

	c.logger.Info("Published order.created event",
		"trace_id", traceID,
		"order_id", order.ID,
		"subject", OrderCreatedSubject)

	return nil
}

func (c *NATSClient) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}

func (c *NATSClient) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}

type OrderCreatedEvent struct {
	OrderID   string           `json:"order_id"`
	UserID    string          `json:"user_id"`
	Status    string          `json:"status"`
	Total     float64         `json:"total"`
	Items     []domain.OrderItem `json:"items"`
	CreatedAt time.Time       `json:"created_at"`
	TraceID   string          `json:"trace_id"`
}

func formatTraceparent(traceID string) string {
	return "00-" + traceID + "-01"
}
