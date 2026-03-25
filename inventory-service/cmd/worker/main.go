package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"

	"github.com/fahri-ris/dops-be.git/inventory-service/internal/config"
	"github.com/fahri-ris/dops-be.git/inventory-service/internal/repository"
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
	logger.Info("Connected to database", "schema", cfg.DBSchema)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		logger.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}

	logger.Info("Connected to database")

	inventoryRepo := repository.NewInventoryRepository(db)

	w := worker.NewWorker(inventoryRepo, logger, cfg.MaxRetries)

	var wg sync.WaitGroup
	jobCh := make(chan worker.OrderMessage, cfg.WorkerCount*2)

	logger.Info("Starting worker pool", "workers", cfg.WorkerCount)

	for i := 0; i < cfg.WorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			logger := logger.With("worker_id", workerID)
			logger.Info("Worker started")

			for msg := range jobCh {
				if err := w.Process(context.Background(), msg); err != nil {
					logger.Error("Failed to process message", "error", err)
				}
			}

			logger.Info("Worker stopped")
		}(i)
	}

	logger.Info("Worker pool running, waiting for messages...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down worker pool...")

	close(jobCh)
	wg.Wait()

	logger.Info("Worker pool stopped")
}
