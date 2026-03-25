package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/fahri-ris/dops-be.git/order-service/internal/domain"
	"github.com/fahri-ris/dops-be.git/order-service/internal/middleware"
	"go.opentelemetry.io/otel/propagation"
)

type PaymentClient struct {
	httpClient *http.Client
	baseURL   string
	logger     *slog.Logger
}

type PaymentError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func NewPaymentClient(certFile, keyFile, caFile, baseURL string, logger *slog.Logger) (*PaymentClient, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:     caCertPool,
		MinVersion:  tls.VersionTLS12,
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	logger.Info("mTLS payment client configured", "base_url", baseURL)

	return &PaymentClient{
		httpClient: httpClient,
		baseURL:   baseURL,
		logger:    logger,
	}, nil
}

func (c *PaymentClient) ProcessPayment(ctx context.Context, req domain.PaymentRequest) (*domain.PaymentResponse, error) {
	traceID := middleware.GetTraceIDContext(ctx)

	url := c.baseURL + "/payment/process"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		c.logger.Error("Failed to create payment request",
			"trace_id", traceID,
			"error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Inject trace context using OTel propagator
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	propagator.Inject(ctx, propagation.HeaderCarrier(httpReq.Header))

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Trace-ID", traceID)

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	httpReq.Body = io.NopCloser(bytes.NewReader(payload))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("Payment request failed",
			"trace_id", traceID,
			"order_id", req.OrderID,
			"error", err)
		return nil, fmt.Errorf("payment request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var paymentErr PaymentError
		if err := json.NewDecoder(resp.Body).Decode(&paymentErr); err != nil {
			return nil, fmt.Errorf("payment failed with status %d", resp.StatusCode)
		}
		c.logger.Warn("Payment processing failed",
			"trace_id", traceID,
			"order_id", req.OrderID,
			"error", paymentErr.Error,
			"message", paymentErr.Message)
		return nil, fmt.Errorf("payment failed: %s - %s", paymentErr.Error, paymentErr.Message)
	}

	var paymentResp domain.PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		c.logger.Error("Failed to decode payment response",
			"trace_id", traceID,
			"error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("Payment processed successfully",
		"trace_id", traceID,
		"order_id", req.OrderID,
		"payment_id", paymentResp.PaymentID,
		"status", paymentResp.Status)

	return &paymentResp, nil
}
