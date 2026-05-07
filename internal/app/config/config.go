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
	Document   DocumentConfig
	AI         AIConfig
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

// DocumentConfig holds document intake runtime limits.
type DocumentConfig struct {
	StoragePath       string
	MaxFilesPerUpload int
	MaxFileBytes      int64
}

// AIConfig holds AI provider settings for extraction adapters.
type AIConfig struct {
	Provider             string
	NVIDIAAPIKey         string
	NVIDIABaseURL        string
	NVIDIAModel          string
	NVIDIAMaxTextRunes   int
	NVIDIARequestTimeout time.Duration
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
		Document: DocumentConfig{
			StoragePath:       envString("RLE_DOCUMENT_STORAGE_PATH", "./data/documents"),
			MaxFilesPerUpload: envInt("RLE_DOCUMENT_MAX_FILES_PER_UPLOAD", 5),
			MaxFileBytes:      envInt64("RLE_DOCUMENT_MAX_FILE_BYTES", 10<<20),
		},
		AI: AIConfig{
			Provider:             envString("RLE_AI_PROVIDER", "disabled"),
			NVIDIAAPIKey:         envString("RLE_NVIDIA_API_KEY", ""),
			NVIDIABaseURL:        envString("RLE_NVIDIA_BASE_URL", "https://integrate.api.nvidia.com/v1"),
			NVIDIAModel:          envString("RLE_NVIDIA_MODEL", "nvidia/llama-3.1-nemotron-nano-8b-v1"),
			NVIDIAMaxTextRunes:   envInt("RLE_NVIDIA_MAX_TEXT_RUNES", 60000),
			NVIDIARequestTimeout: envDuration("RLE_NVIDIA_REQUEST_TIMEOUT", 45*time.Second),
		},
	}
}

func envString(key, fallback string) string {
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

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
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
