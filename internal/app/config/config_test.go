package config

import "testing"

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv("RLE_HTTP_ADDR", ":18080")
	t.Setenv("RLE_GRPC_ADDR", ":19090")
	t.Setenv("RLE_SQLITE_PATH", "/data/revenue.db")
	t.Setenv("RLE_MIGRATIONS_PATH", "/app/migrations")
	t.Setenv("RLE_LOG_LEVEL", "debug")
	t.Setenv("RLE_LOG_JSON", "false")
	t.Setenv("RLE_KAFKA_BROKERS", "kafka:29092,localhost:9092")
	t.Setenv("RLE_KAFKA_GROUP_ID", "rle-workers")
	t.Setenv("RLE_DOCUMENT_STORAGE_PATH", "/data/documents")
	t.Setenv("RLE_DOCUMENT_MAX_FILES_PER_UPLOAD", "3")
	t.Setenv("RLE_DOCUMENT_MAX_FILE_BYTES", "1048576")
	t.Setenv("RLE_AI_PROVIDER", "nvidia")
	t.Setenv("RLE_NVIDIA_API_KEY", "secret")
	t.Setenv("RLE_NVIDIA_BASE_URL", "https://example.test/v1")
	t.Setenv("RLE_NVIDIA_MODEL", "nvidia/test")
	t.Setenv("RLE_NVIDIA_MAX_TEXT_RUNES", "12000")
	t.Setenv("RLE_NVIDIA_REQUEST_TIMEOUT", "10s")

	cfg := Load()

	if cfg.HTTP.Addr != ":18080" {
		t.Fatalf("HTTP.Addr = %q, want :18080", cfg.HTTP.Addr)
	}
	if cfg.GRPC.Addr != ":19090" {
		t.Fatalf("GRPC.Addr = %q, want :19090", cfg.GRPC.Addr)
	}
	if cfg.SQLite.Path != "/data/revenue.db" {
		t.Fatalf("SQLite.Path = %q, want /data/revenue.db", cfg.SQLite.Path)
	}
	if cfg.Migrations.Path != "/app/migrations" {
		t.Fatalf("Migrations.Path = %q, want /app/migrations", cfg.Migrations.Path)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if cfg.Logging.IsJSON {
		t.Fatal("Logging.IsJSON = true, want false")
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "kafka:29092" || cfg.Kafka.Brokers[1] != "localhost:9092" {
		t.Fatalf("Kafka.Brokers = %#v, want kafka and localhost brokers", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.GroupID != "rle-workers" {
		t.Fatalf("Kafka.GroupID = %q, want rle-workers", cfg.Kafka.GroupID)
	}
	if cfg.Document.StoragePath != "/data/documents" {
		t.Fatalf("Document.StoragePath = %q, want /data/documents", cfg.Document.StoragePath)
	}
	if cfg.Document.MaxFilesPerUpload != 3 {
		t.Fatalf("Document.MaxFilesPerUpload = %d, want 3", cfg.Document.MaxFilesPerUpload)
	}
	if cfg.Document.MaxFileBytes != 1048576 {
		t.Fatalf("Document.MaxFileBytes = %d, want 1048576", cfg.Document.MaxFileBytes)
	}
	if cfg.AI.Provider != "nvidia" {
		t.Fatalf("AI.Provider = %q, want nvidia", cfg.AI.Provider)
	}
	if cfg.AI.NVIDIAAPIKey != "secret" {
		t.Fatalf("AI.NVIDIAAPIKey = %q, want secret", cfg.AI.NVIDIAAPIKey)
	}
	if cfg.AI.NVIDIABaseURL != "https://example.test/v1" {
		t.Fatalf("AI.NVIDIABaseURL = %q, want example url", cfg.AI.NVIDIABaseURL)
	}
	if cfg.AI.NVIDIAModel != "nvidia/test" {
		t.Fatalf("AI.NVIDIAModel = %q, want nvidia/test", cfg.AI.NVIDIAModel)
	}
	if cfg.AI.NVIDIAMaxTextRunes != 12000 {
		t.Fatalf("AI.NVIDIAMaxTextRunes = %d, want 12000", cfg.AI.NVIDIAMaxTextRunes)
	}
	if cfg.AI.NVIDIARequestTimeout.String() != "10s" {
		t.Fatalf("AI.NVIDIARequestTimeout = %s, want 10s", cfg.AI.NVIDIARequestTimeout)
	}
}
