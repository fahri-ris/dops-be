package main

import (
	"context"
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
	"github.com/redis/go-redis/v9"

	"github.com/fahri-ris/dops-be.git/order-service/internal/broker"
	"github.com/fahri-ris/dops-be.git/order-service/internal/client"
	"github.com/fahri-ris/dops-be.git/order-service/internal/config"
	"github.com/fahri-ris/dops-be.git/order-service/internal/handler"
	"github.com/fahri-ris/dops-be.git/order-service/internal/metrics"
	"github.com/fahri-ris/dops-be.git/order-service/internal/middleware"
	"github.com/fahri-ris/dops-be.git/order-service/internal/repository"
	"github.com/fahri-ris/dops-be.git/order-service/internal/service"
	"github.com/fahri-ris/dops-be.git/order-service/internal/tracing"
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

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		logger.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("Failed to ping Redis", "error", err)
		os.Exit(1)
	}

	logger.Info("Connected to database and Redis")

	// Initialize NATS client for event publishing
	natsClient, err := broker.NewNATSClient(cfg.NATSURL, logger)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	// Initialize OpenTelemetry tracer
	tracer, err := tracing.NewTracer("order-service")
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

	// Initialize Payment client with mTLS
	paymentClient, err := client.NewPaymentClient(
		cfg.MTLSCert,
		cfg.MTLSKey,
		cfg.MTLSCert, // Using cert as CA for now - should be separate CA file in production
		cfg.PaymentServiceURL,
		logger,
	)
	if err != nil {
		logger.Error("Failed to initialize payment client", "error", err)
		os.Exit(1)
	}

	orderRepo := repository.NewOrderRepository(db)
	orderItemRepo := repository.NewOrderItemRepository(db)
	productRepo := repository.NewProductRepository(db, rdb)

	orderService := service.NewOrderService(
		orderRepo,
		orderItemRepo,
		productRepo,
		paymentClient,
		natsClient,
		logger,
	)

	orderHandler := handler.NewOrderHandler(orderService, logger)
	authHandler := handler.NewAuthHandler(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, logger)
	healthHandler := handler.NewHealthzHandler(db, rdb)

	traceMiddleware := middleware.TraceMiddleware(logger)
	jwtMiddleware := middleware.NewJWTMiddleware(cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTSecret, logger)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(
		rdb,
		"rate_limit:",
		cfg.RateLimitRequests,
		time.Duration(cfg.RateLimitWindowSec)*time.Second,
		logger,
	)
	metricsMiddleware := metrics.Middleware("order-service")

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthHandler.Liveness)
	mux.HandleFunc("/readyz", healthHandler.Readiness)
	mux.Handle("/metrics", promhttp.Handler())

	authHandlerWithTrace := traceMiddleware(http.HandlerFunc(authHandler.Login))
	mux.Handle("/api/v1/auth/login", authHandlerWithTrace)

	var ordersHandler http.Handler = http.HandlerFunc(orderHandler.CreateOrder)
	ordersHandler = metricsMiddleware(ordersHandler)
	ordersHandler = traceMiddleware(ordersHandler)
	ordersHandler = jwtMiddleware.Authenticate()(ordersHandler)
	ordersHandler = rateLimitMiddleware.Limit()(ordersHandler)
	mux.Handle("/api/v1/orders", ordersHandler)

	var getOrderHandler http.Handler = http.HandlerFunc(orderHandler.GetOrder)
	getOrderHandler = metricsMiddleware(getOrderHandler)
	getOrderHandler = traceMiddleware(getOrderHandler)
	getOrderHandler = jwtMiddleware.Authenticate()(getOrderHandler)
	getOrderHandler = rateLimitMiddleware.Limit()(getOrderHandler)
	mux.Handle("/api/v1/orders/", getOrderHandler)

	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: mux,
	}

	go func() {
		logger.Info("Starting server", "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

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
