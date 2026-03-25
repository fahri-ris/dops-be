package config

import (
	"context"
	"log/slog"
	"os"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	// Server
	ServerPort string `env:"SERVER_PORT, default=8080"`

	// Database
	DBHost     string `env:"DB_HOST, default=localhost"`
	DBPort     string `env:"DB_PORT, default=5432"`
	DBUser     string `env:"DB_USER, default=postgres"`
	DBPassword string `env:"DB_PASSWORD"`
	DBName     string `env:"DB_NAME, default=dops"`
	DBSchema   string `env:"DB_SCHEMA, default=orders"`

	// Redis
	RedisAddr string `env:"REDIS_ADDR, default=localhost:6379"`

	// NATS
	NATSURL string `env:"NATS_URL, default=nats://localhost:4222"`

	// JWT
	JWTIssuer   string `env:"JWT_ISSUER"`
	JWTAudience string `env:"JWT_AUDIENCE"`
	JWTSecret   string `env:"JWT_SECRET"`

	// mTLS
	MTLSCert string `env:"MTLS_CERT"`
	MTLSKey  string `env:"MTLS_KEY"`

	// Payment Service
	PaymentServiceURL string `env:"PAYMENT_SERVICE_URL, default=https://localhost:8081"`

	// Rate Limiting
	RateLimitRequests  int `env:"RATE_LIMIT_REQUESTS, default=100"`
	RateLimitWindowSec int `env:"RATE_LIMIT_WINDOW_SEC, default=60"`
}

func Load() *Config {
	ctx := context.Background()
	cfg := &Config{}

	if err := envconfig.Process(ctx, cfg); err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	return cfg
}

func (c *Config) DBConnectionString() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=disable"
}
