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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/lib/pq"
	"github.com/joho/godotenv"

	"github.com/fahri-ris/dops-be.git/payment-service/internal/config"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/handler"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/metrics"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/middleware"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/repository"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/service"
	"github.com/fahri-ris/dops-be.git/payment-service/internal/tracing"
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

	// Initialize OpenTelemetry tracer
	tracer, err := tracing.NewTracer("payment-service")
	if err != nil {
		logger.Error("Failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tracer.Shutdown(ctx)
	}()
	logger.Info("OpenTelemetry tracer initialized")

	paymentRepo := repository.NewPaymentRepository(db)

	paymentService := service.NewPaymentService(paymentRepo, logger)

	paymentHandler := handler.NewPaymentHandler(paymentService, logger)
	healthHandler := handler.NewHealthzHandler(db)

	traceMiddleware := middleware.TraceMiddleware(logger)
	metricsMiddleware := metrics.Middleware("payment-service")

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthHandler.Liveness)
	mux.HandleFunc("/readyz", healthHandler.Readiness)
	mux.Handle("/metrics", promhttp.Handler())

	var paymentHandlerWithMiddleware http.Handler = http.HandlerFunc(paymentHandler.ProcessPayment)
	paymentHandlerWithMiddleware = metricsMiddleware(paymentHandlerWithMiddleware)
	paymentHandlerWithMiddleware = traceMiddleware(paymentHandlerWithMiddleware)
	mux.Handle("/payment/process", paymentHandlerWithMiddleware)

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
