package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	OrdersProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_processed_total",
			Help: "Total number of orders processed by the inventory service",
		},
		[]string{"status"},
	)

	OrderProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "order_processing_duration_seconds",
			Help:    "Duration of order processing in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	InventoryUpdateTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inventory_update_total",
			Help: "Total number of inventory updates",
		},
		[]string{"product_id", "status"},
	)

	WorkerPoolSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "inventory_worker_pool_size",
			Help: "Current size of the worker pool",
		},
	)

	ActiveWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "inventory_active_workers",
			Help: "Number of currently active workers",
		},
	)

	DLQMessagesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dlq_messages_total",
			Help: "Total number of messages sent to dead letter queue",
		},
	)

	RetryAttemptsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "retry_attempts_total",
			Help: "Total number of retry attempts",
		},
	)
)

func RecordOrderProcessed(status string) {
	OrdersProcessedTotal.WithLabelValues(status).Inc()
}

func RecordInventoryUpdate(productID, status string) {
	InventoryUpdateTotal.WithLabelValues(productID, status).Inc()
}

func SetWorkerPoolSize(size int) {
	WorkerPoolSize.Set(float64(size))
}

func SetActiveWorkers(count int) {
	ActiveWorkers.Set(float64(count))
}

func RecordDLQMessage() {
	DLQMessagesTotal.Inc()
}

func RecordRetryAttempt() {
	RetryAttemptsTotal.Inc()
}
