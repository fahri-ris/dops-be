package broker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/fahri-ris/dops-be.git/inventory-service/internal/metrics"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/worker"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/trace"
)

type Publisher struct {
	nc       *nats.Conn
	js       nats.JetStreamContext
	dlqTopic string
	tracer   trace.Tracer
	logger   *slog.Logger
}

func NewPublisher(nc *nats.Conn, tracer trace.Tracer, logger *slog.Logger) (*Publisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	return &Publisher{
		nc:       nc,
		js:       js,
		dlqTopic: "order.failed",
		tracer:   tracer,
		logger:   logger,
	}, nil
}

func (p *Publisher) PublishToDLQ(ctx context.Context, msg worker.OrderMessage, reason string) error {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()

	dlqMsg := DLQMessage{
		OriginalMessage: msg,
		FailureReason:   reason,
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      0,
		TraceID:         traceID,
	}

	data, err := json.Marshal(dlqMsg)
	if err != nil {
		p.logger.Error("Failed to marshal DLQ message", "error", err)
		return err
	}

	_, err = p.js.Publish(p.dlqTopic, data)
	if err != nil {
		p.logger.Error("Failed to publish to DLQ", "error", err, "topic", p.dlqTopic)
		metrics.RecordDLQMessage()
		return err
	}

	metrics.RecordDLQMessage()

	p.logger.Warn("Message published to DLQ",
		"order_id", msg.OrderID,
		"topic", p.dlqTopic,
		"reason", reason,
		"trace_id", traceID,
	)

	return nil
}

func (p *Publisher) Close() error {
	p.nc.Close()
	return nil
}

type DLQMessage struct {
	OriginalMessage worker.OrderMessage `json:"original_message"`
	FailureReason   string              `json:"failure_reason"`
	FailedAt        string              `json:"failed_at"`
	RetryCount      int                 `json:"retry_count"`
	TraceID         string              `json:"trace_id"`
}
