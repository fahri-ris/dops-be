package config

import (
	"context"
	"log/slog"
	"os"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	// Server
	ServerPort string `env:"SERVER_PORT, default=8081"`

	// Database
	DBHost     string `env:"DB_HOST, default=localhost"`
	DBPort     string `env:"DB_PORT, default=5432"`
	DBUser     string `env:"DB_USER, default=postgres"`
	DBPassword string `env:"DB_PASSWORD"`
	DBName     string `env:"DB_NAME, default=dops"`
	DBSchema   string `env:"DB_SCHEMA, default=payments"`

	// mTLS (required)
	MTLSCert string `env:"MTLS_CERT, required"`
	MTLSKey  string `env:"MTLS_KEY, required"`
	MTLSCA   string `env:"MTLS_CA_CERT, required"` // CA cert to verify client certificates
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
