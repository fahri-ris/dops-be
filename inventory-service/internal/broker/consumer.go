package broker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/fahri-ris/dops-be.git/inventory-service/internal/metrics"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/tracing"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/worker"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Consumer struct {
	nc      *nats.Conn
	js      nats.JetStreamContext
	subject string
	queue   string
	tracer  trace.Tracer
	logger  *slog.Logger
}

type ConsumerOption func(*Consumer)

func WithQueue(queue string) ConsumerOption {
	return func(c *Consumer) {
		c.queue = queue
	}
}

func NewConsumer(nc *nats.Conn, tracer trace.Tracer, logger *slog.Logger, opts ...ConsumerOption) (*Consumer, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	c := &Consumer{
		nc:      nc,
		js:      js,
		subject: "order.created",
		tracer:  tracer,
		logger:  logger,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (c *Consumer) Subscribe(ctx context.Context, handler func(context.Context, worker.OrderMessage) error) error {
	_, err := c.js.Subscribe(c.subject, func(msg *nats.Msg) {
		startTime := time.Now()
		headers := make(map[string]string)
		for k, v := range msg.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		traceID := tracing.ExtractTraceIDFromMap(headers)
		logger := c.logger.With(
			"trace_id", traceID,
			"subject", msg.Subject,
		)

		logger.Info("Received message")

		spanCtx, span := tracing.StartSpan(ctx, c.tracer, "inventory.consume",
			trace.WithAttributes(
				attribute.String("messaging.system", "nats"),
				attribute.String("messaging.destination", c.subject),
				attribute.String("trace_id", traceID),
			),
		)
		defer span.End()

		var orderMsg worker.OrderMessage
		if err := json.Unmarshal(msg.Data, &orderMsg); err != nil {
			logger.Error("Failed to unmarshal message", "error", err)
			msg.Nak()
			return
		}

		orderMsg.TraceID = traceID

		if err := handler(spanCtx, orderMsg); err != nil {
			logger.Error("Failed to process message", "error", err)
			metrics.RecordOrderProcessed("failed")
			msg.Nak()
			return
		}

		duration := time.Since(startTime).Seconds()
		metrics.OrderProcessingDuration.WithLabelValues("success").Observe(duration)
		metrics.RecordOrderProcessed("success")

		msg.Ack()
		logger.Info("Message acknowledged", "duration_seconds", duration)
	}, nats.DeliverAll())

	if err != nil {
		return err
	}

	c.logger.Info("Subscribed to subject", "subject", c.subject)
	return nil
}

func (c *Consumer) Close() error {
	c.nc.Close()
	return nil
}
