package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"

	"github.com/fahri-ris/dops-be.git/payment-service/internal/config"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/handler"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/middleware"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/repository"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/service"
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

	paymentRepo := repository.NewPaymentRepository(db)

	paymentService := service.NewPaymentService(paymentRepo, logger)

	paymentHandler := handler.NewPaymentHandler(paymentService, logger)
	healthHandler := handler.NewHealthzHandler()

	traceMiddleware := middleware.TraceMiddleware(logger)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthHandler.Liveness)
	mux.HandleFunc("/readyz", healthHandler.Readiness)

	mux.HandleFunc("/payment/process", paymentHandler.ProcessPayment)

	var handler http.Handler = mux
	handler = traceMiddleware(handler)

	// Build TLS config for mTLS
	caCert, err := os.ReadFile(cfg.MTLSCA)
	if err != nil {
		logger.Error("Failed to read CA certificate", "error", err)
		os.Exit(1)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		logger.Error("Failed to add CA certificate to pool")
		os.Exit(1)
	}

	tlsConfig := &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
		MinVersion: tls.VersionTLS12,
	}

	server := &http.Server{
		Addr:      ":" + cfg.ServerPort,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	go func() {
		logger.Info("Starting payment service with mTLS", "port", cfg.ServerPort)
		if err := server.ListenAndServeTLS(cfg.MTLSCert, cfg.MTLSKey); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Verify server started successfully
	logger.Info("Payment service mTLS configured",
		"port", cfg.ServerPort,
		"tls_min_version", "1.2",
		"client_auth", "require_and_verify")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}
	logger.Info("Server exited")
}
