package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	platformsqlite "github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	"github.com/google/uuid"
)

func TestSQLiteStoreAppendAndListPending(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestSQLite(t, ctx)
	store := newTestStore(t, db)
	envelope := validEnvelope(t)

	if err := store.Append(ctx, envelope); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	records, err := store.ListPending(ctx, ListPendingCommand{
		AvailableAt: envelope.OccurredAt.Add(time.Second),
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}

	if got := len(records); got != 1 {
		t.Fatalf("pending count = %d, want 1", got)
	}

	record := records[0]
	if record.ID != envelope.EventID {
		t.Fatalf("event id = %s, want %s", record.ID, envelope.EventID)
	}
	if record.PartitionKey != envelope.ContractID.String() {
		t.Fatalf("partition key = %q, want %q", record.PartitionKey, envelope.ContractID)
	}
	if record.Headers["event_id"] != envelope.EventID.String() {
		t.Fatalf("event_id header = %q, want %q", record.Headers["event_id"], envelope.EventID)
	}

	var decoded appevent.Envelope
	if err := json.Unmarshal(record.PayloadJSON, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded.Type != appevent.TypeReconciliationRunRequested {
		t.Fatalf("event type = %q, want %q", decoded.Type, appevent.TypeReconciliationRunRequested)
	}
}

func TestSQLiteStoreMarkPublishedRemovesFromPending(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestSQLite(t, ctx)
	store := newTestStore(t, db)
	envelope := validEnvelope(t)

	if err := store.Append(ctx, envelope); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if err := store.MarkPublished(ctx, envelope.EventID, envelope.OccurredAt.Add(time.Minute)); err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}

	records, err := store.ListPending(ctx, ListPendingCommand{
		AvailableAt: envelope.OccurredAt.Add(time.Hour),
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}

	if len(records) != 0 {
		t.Fatalf("pending count = %d, want 0", len(records))
	}
}

func TestSQLiteStoreMarkFailedSchedulesRetry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestSQLite(t, ctx)
	store := newTestStore(t, db)
	envelope := validEnvelope(t)
	retryAt := envelope.OccurredAt.Add(5 * time.Minute)

	if err := store.Append(ctx, envelope); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if err := store.MarkFailed(ctx, envelope.EventID, "broker unavailable", retryAt); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	early, err := store.ListPending(ctx, ListPendingCommand{
		AvailableAt: retryAt.Add(-time.Second),
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListPending() early error = %v", err)
	}
	if len(early) != 0 {
		t.Fatalf("early pending count = %d, want 0", len(early))
	}

	records, err := store.ListPending(ctx, ListPendingCommand{
		AvailableAt: retryAt,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListPending() retry error = %v", err)
	}
	if got := len(records); got != 1 {
		t.Fatalf("retry pending count = %d, want 1", got)
	}
	if records[0].Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", records[0].Attempts)
	}
	if records[0].LastError != "broker unavailable" {
		t.Fatalf("last error = %q, want broker unavailable", records[0].LastError)
	}
}

func TestSQLiteStoreInboxIdempotency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestSQLite(t, ctx)
	store := newTestStore(t, db)
	eventID := uuid.New()

	processed, err := store.AlreadyProcessed(ctx, eventID, "usage-consumer")
	if err != nil {
		t.Fatalf("AlreadyProcessed() before error = %v", err)
	}
	if processed {
		t.Fatal("AlreadyProcessed() before = true, want false")
	}

	err = store.MarkProcessed(ctx, InboxRecord{
		EventID:     eventID,
		Handler:     "usage-consumer",
		Topic:       appevent.TopicUsageRecordsV1,
		SourceKey:   "metering:usage-1",
		ProcessedAt: time.Date(2026, time.May, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("MarkProcessed() error = %v", err)
	}

	if err := store.MarkProcessed(ctx, InboxRecord{EventID: eventID, Handler: "usage-consumer"}); err != nil {
		t.Fatalf("MarkProcessed() duplicate error = %v", err)
	}

	processed, err = store.AlreadyProcessed(ctx, eventID, "usage-consumer")
	if err != nil {
		t.Fatalf("AlreadyProcessed() after error = %v", err)
	}
	if !processed {
		t.Fatal("AlreadyProcessed() after = false, want true")
	}
}

func TestSQLiteStoreRejectsInvalidPendingLimit(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, openTestSQLite(t, context.Background()))
	_, err := store.ListPending(context.Background(), ListPendingCommand{Limit: -1})
	if !errors.Is(err, ErrLimitInvalid) {
		t.Fatalf("ListPending() error = %v, want %v", err, ErrLimitInvalid)
	}
}

func validEnvelope(t *testing.T) appevent.Envelope {
	t.Helper()

	tenantID := uuid.New()
	contractID := uuid.New()
	envelope, err := appevent.NewEnvelope(appevent.NewEnvelopeCommand{
		EventID:    uuid.New(),
		Type:       appevent.TypeReconciliationRunRequested,
		Topic:      appevent.TopicReconciliationRunRequestedV1,
		OccurredAt: time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
		TenantID:   tenantID,
		ContractID: contractID,
		TraceID:    "trace-outbox",
		Payload: appevent.ReconciliationRunRequestedPayload{
			TenantID:   tenantID,
			ContractID: contractID,
			Period: appevent.BillingPeriodPayload{
				Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
			},
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	return envelope
}

func newTestStore(t *testing.T, db *sql.DB) *SQLiteStore {
	t.Helper()

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	return store
}

func openTestSQLite(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	db, err := platformsqlite.Open(
		ctx,
		"file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared&_pragma=foreign_keys(1)",
	)
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	schema, err := os.ReadFile(sqliteSchemaPath(t))
	if err != nil {
		t.Fatalf("read sqlite schema: %v", err)
	}

	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		t.Fatalf("apply sqlite schema: %v", err)
	}

	return db
}

func sqliteSchemaPath(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file path")
	}

	return filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"platform",
		"sqlite",
		"schema",
		"schema.sql",
	)
}
