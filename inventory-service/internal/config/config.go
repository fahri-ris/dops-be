package config

import (
	"context"
	"log/slog"
	"os"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	// Database
	DBHost     string `env:"DB_HOST, default=localhost"`
	DBPort     string `env:"DB_PORT, default=5432"`
	DBUser     string `env:"DB_USER, default=postgres"`
	DBPassword string `env:"DB_PASSWORD"`
	DBName     string `env:"DB_NAME, default=dops"`
	DBSchema   string `env:"DB_SCHEMA, default=inventory"`

	// NATS
	NATSURL string `env:"NATS_URL, default=nats://localhost:4222"`

	// Worker
	WorkerCount int `env:"WORKER_COUNT, default=10"`
	MaxRetries  int `env:"MAX_RETRIES, default=3"`
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
