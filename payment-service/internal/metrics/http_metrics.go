package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts total HTTP requests by service, endpoint, and status code
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "endpoint", "status_code"},
	)

	// HTTPRequestDurationSeconds tracks request duration by service and endpoint
	HTTPRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"service", "endpoint"},
	)
)

// RecordRequest records an HTTP request metric
func RecordRequest(service, endpoint, statusCode string, duration time.Duration) {
	HTTPRequestsTotal.WithLabelValues(service, endpoint, statusCode).Inc()
	HTTPRequestDurationSeconds.WithLabelValues(service, endpoint).Observe(duration.Seconds())
}

// Middleware returns a Prometheus middleware function for HTTP handlers
func Middleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

			next.ServeHTTP(wrapped, r.WithContext(r.Context()))

			duration := time.Since(start)
			statusCode := strconv.Itoa(wrapped.statusCode)
			endpoint := r.URL.Path

			RecordRequest(serviceName, endpoint, statusCode, duration)
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
