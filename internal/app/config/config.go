package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the application configuration.
type Config struct {
	HTTP       HTTPConfig
	GRPC       GRPCConfig
	SQLite     SQLiteConfig
	Migrations MigrationConfig
	Logging    LoggingConfig
	Kafka      KafkaConfig
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

// KafkaConfig holds Kafka integration settings.
type KafkaConfig struct {
	Brokers []string
	GroupID string
}

// Load returns the application configuration.
func Load() Config {
	return Config{
		HTTP: HTTPConfig{
			Addr:              envString("RLE_HTTP_ADDR", ":8080"),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			ShutdownTimeout:   10 * time.Second,
			RateLimit:         100,
			RateLimitWindow:   time.Minute,
		},
		GRPC: GRPCConfig{
			Addr:            envString("RLE_GRPC_ADDR", ":9090"),
			ShutdownTimeout: 10 * time.Second,
		},
		SQLite: SQLiteConfig{
			Path: envString("RLE_SQLITE_PATH", "./local.db"),
		},
		Migrations: MigrationConfig{
			Path: envString("RLE_MIGRATIONS_PATH", "internal/platform/sqlite/migrations"),
		},
		Logging: LoggingConfig{
			Level:  envString("RLE_LOG_LEVEL", ""),
			IsJSON: envBool("RLE_LOG_JSON", true),
		},
		Kafka: KafkaConfig{
			Brokers: envStringSlice("RLE_KAFKA_BROKERS", []string{"localhost:9092"}),
			GroupID: envString("RLE_KAFKA_GROUP_ID", "revenueleakageengine"),
		},
	}
}

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envStringSlice(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return fallback
	}

	return result
}
