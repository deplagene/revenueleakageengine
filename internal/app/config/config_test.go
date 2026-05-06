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
}
