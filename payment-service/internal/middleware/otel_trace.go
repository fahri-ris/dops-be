package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/fahri-ris/dops-be.git/payment-service/internal/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceMiddleware returns an OpenTelemetry trace middleware
func TraceMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	propagator := otel.GetTextMapPropagator()
	tracer := otel.Tracer("payment-service")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request headers (traceparent from Order Service)
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start a new span
			ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request with the trace context
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			duration := time.Since(start)

			// Record span attributes
			span.SetAttributes(
				attribute.Int("http.status_code", wrapped.statusCode),
				attribute.Int64("http.duration_ms", duration.Milliseconds()),
			)

			// Log with trace ID
			traceID := tracing.ExtractTraceID(ctx)
			logger.Info("HTTP request",
				"trace_id", traceID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration", duration,
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// InjectTraceContext injects trace context into the request headers for propagation
func InjectTraceContext(ctx context.Context, req *http.Request) {
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// GetTraceIDContext extracts trace ID from context
func GetTraceIDContext(ctx context.Context) string {
	return tracing.ExtractTraceID(ctx)
}
