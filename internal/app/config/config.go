package config

import (
	"time"
)

// TODO: add config load logic from .env or yaml file
// TODO: add methods to hide sensitive information (logging)

// Config represents the application configuration.
type Config struct {
	HTTP       HTTPConfig
	GRPC       GRPCConfig
	SQLite     SQLiteConfig
	Migrations MigrationConfig
	Logging    LoggingConfig
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	ShutdownTimeout   time.Duration
	RateLimit         int
	RateLimitWindow   time.Duration
}

// GRPCConfig holds gRPC server settings.
type GRPCConfig struct {
	Addr            string
	ShutdownTimeout time.Duration
}

// SQLiteConfig holds SQLite database settings.
type SQLiteConfig struct {
	Path string
}

// MigrationConfig holds migration settings.
type MigrationConfig struct {
	Path string
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string
	IsJSON bool
}

// Load returns the application configuration.
func Load() Config {
	return Config{
		HTTP: HTTPConfig{
			Addr:              ":8080",
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			ShutdownTimeout:   10 * time.Second,
			RateLimit:         100,
			RateLimitWindow:   time.Minute,
		},
		GRPC: GRPCConfig{
			Addr:            ":9090",
			ShutdownTimeout: 10 * time.Second,
		},
		SQLite: SQLiteConfig{
			Path: "./local.db",
		},
		Migrations: MigrationConfig{
			Path: "internal/platform/sqlite/migrations",
		},
		Logging: LoggingConfig{
			Level:  "",
			IsJSON: true,
		},
	}
}
