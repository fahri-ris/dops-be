package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fahri-ris/dops-be.git/inventory-service/internal/broker"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/config"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/metrics"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/repository"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/tracing"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/worker"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found, using environment variables")
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()

	// Initialize tracer
	tracer, err := tracing.NewTracer("inventory-service")
	if err != nil {
		logger.Error("Failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracer.Shutdown(ctx); err != nil {
			logger.Error("Failed to shutdown tracer", "error", err)
		}
	}()

	// Connect to database
	db, err := sql.Open("postgres", cfg.DBConnectionString())
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if _, err := db.Exec("SET search_path TO " + cfg.DBSchema); err != nil {
		logger.Error("Failed to set search_path", "error", err, "schema", cfg.DBSchema)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		logger.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}

	logger.Info("Connected to database", "schema", cfg.DBSchema)

	// Connect to NATS
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	logger.Info("Connected to NATS", "url", cfg.NATSURL)

	// Initialize repository
	inventoryRepo := repository.NewInventoryRepository(db)

	// Initialize worker
	w := worker.NewWorker(inventoryRepo, logger, cfg.MaxRetries)

	// Initialize publisher for DLQ
	publisher, err := broker.NewPublisher(nc, tracer.Tracer(), logger)
	if err != nil {
		logger.Error("Failed to create publisher", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	// Worker pool setup
	var wg sync.WaitGroup
	jobCh := make(chan worker.OrderMessage, cfg.WorkerCount*2)

	metrics.SetWorkerPoolSize(cfg.WorkerCount)

	logger.Info("Starting worker pool", "workers", cfg.WorkerCount)

	// Start worker goroutines
	for i := 0; i < cfg.WorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			logger := logger.With("worker_id", workerID)
			metrics.SetActiveWorkers(i + 1)
			logger.Info("Worker started")

			for msg := range jobCh {
				ctx := context.Background()
				if err := w.ProcessWithRetry(ctx, msg, publisher); err != nil {
					logger.Error("Failed to process message", "error", err)
				}
			}

			metrics.SetActiveWorkers(i - 1)
			logger.Info("Worker stopped")
		}(i)
	}

	// Initialize consumer
	consumer, err := broker.NewConsumer(nc, tracer.Tracer(), logger)
	if err != nil {
		logger.Error("Failed to create consumer", "error", err)
		os.Exit(1)
	}

	// Start consuming messages
	go func() {
		if err := consumer.Subscribe(context.Background(), func(ctx context.Context, msg worker.OrderMessage) error {
			select {
			case jobCh <- msg:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}); err != nil {
			logger.Error("Failed to subscribe", "error", err)
		}
	}()

	// Health and metrics server
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/readyz", readyz(db))
	mux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:    ":9090",
		Handler: mux,
	}

	go func() {
		logger.Info("Starting metrics server on :9090")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Metrics server failed", "error", err)
		}
	}()

	logger.Info("Worker pool running, waiting for messages...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down worker pool...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop accepting new jobs
	close(jobCh)

	// Wait for workers to finish
	wg.Wait()

	// Shutdown metrics server
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Failed to shutdown metrics server", "error", err)
	}

	// Close consumer
	consumer.Close()

	logger.Info("Worker pool stopped")
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func readyz(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "Database not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	}
}
