package ingestion

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/deplagene/revenueleakageengine/internal/migrator"
	"github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	"github.com/google/uuid"
)

func TestSQLiteStoreUpsertUsageRecordsIsIdempotentByExternalID(t *testing.T) {
	ctx := context.Background()
	store, db := newMigratedSQLiteStore(t, ctx)
	ids := insertIngestionFixture(t, ctx, db)

	record := sqliteUsageRecord(ids)
	if err := store.UpsertUsageRecords(ctx, []billing.UsageRecord{record}); err != nil {
		t.Fatalf("UpsertUsageRecords() error = %v", err)
	}

	record.ID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	record.Quantity = 250
	if err := store.UpsertUsageRecords(ctx, []billing.UsageRecord{record}); err != nil {
		t.Fatalf("UpsertUsageRecords() second call error = %v", err)
	}

	var count int
	var quantity int64
	err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*), MAX(quantity)
		 FROM usage_records
		 WHERE tenant_id = ? AND source_system = ? AND external_id = ?`,
		ids.tenantID.String(),
		record.SourceSystem,
		record.ExternalID,
	).Scan(&count, &quantity)
	if err != nil {
		t.Fatalf("query usage records: %v", err)
	}

	if count != 1 {
		t.Fatalf("usage record count = %d, want 1", count)
	}

	if quantity != 250 {
		t.Fatalf("usage quantity = %d, want 250", quantity)
	}
}

func TestSQLiteStoreUpsertInvoiceIsIdempotentByExternalID(t *testing.T) {
	ctx := context.Background()
	store, db := newMigratedSQLiteStore(t, ctx)
	ids := insertIngestionFixture(t, ctx, db)

	invoice := sqliteInvoice(t, ids)
	line := sqliteInvoiceLine(invoice.ID, ids.billableItemID, 10000)
	if err := store.UpsertInvoice(ctx, invoice, []billing.InvoiceLine{line}); err != nil {
		t.Fatalf("UpsertInvoice() error = %v", err)
	}

	replayedInvoice := invoice
	replayedInvoice.ID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	replayedInvoice.TotalAmount = valueobject.MustMoney("USD", 12000)
	replayedLine := sqliteInvoiceLine(replayedInvoice.ID, ids.billableItemID, 12000)
	replayedLine.ID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	if err := store.UpsertInvoice(ctx, replayedInvoice, []billing.InvoiceLine{replayedLine}); err != nil {
		t.Fatalf("UpsertInvoice() replay error = %v", err)
	}

	var invoiceCount int
	var persistedInvoiceID string
	var totalAmount int64
	err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*), MIN(id), MAX(total_amount_minor_units)
		 FROM invoices
		 WHERE tenant_id = ? AND source_system = ? AND external_id = ?`,
		ids.tenantID.String(),
		invoice.SourceSystem,
		invoice.ExternalID,
	).Scan(&invoiceCount, &persistedInvoiceID, &totalAmount)
	if err != nil {
		t.Fatalf("query invoices: %v", err)
	}

	if invoiceCount != 1 {
		t.Fatalf("invoice count = %d, want 1", invoiceCount)
	}

	if totalAmount != 12000 {
		t.Fatalf("invoice total amount = %d, want 12000", totalAmount)
	}

	var lineCount int
	var lineTotal int64
	err = db.QueryRowContext(
		ctx,
		`SELECT COUNT(*), MAX(line_total_minor_units)
		 FROM invoice_lines
		 WHERE invoice_id = ?`,
		persistedInvoiceID,
	).Scan(&lineCount, &lineTotal)
	if err != nil {
		t.Fatalf("query invoice lines: %v", err)
	}

	if lineCount != 1 {
		t.Fatalf("invoice line count = %d, want 1", lineCount)
	}

	if lineTotal != 12000 {
		t.Fatalf("invoice line total = %d, want 12000", lineTotal)
	}
}

type ingestionFixtureIDs struct {
	tenantID       uuid.UUID
	customerID     uuid.UUID
	contractID     uuid.UUID
	billableItemID uuid.UUID
}

func newMigratedSQLiteStore(t *testing.T, ctx context.Context) (*SQLiteStore, *sql.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ingestion.db")
	db, err := sqlite.Open(ctx, sqlite.LocalDSN(dbPath))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	migrationsPath := filepath.Join("..", "..", "platform", "sqlite", "migrations")
	m := migrator.NewMigrator(db, migrationsPath)
	if err := m.Up(ctx); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return NewSQLiteStore(db), db
}

func insertIngestionFixture(t *testing.T, ctx context.Context, db *sql.DB) ingestionFixtureIDs {
	t.Helper()

	ids := ingestionFixtureIDs{
		tenantID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		customerID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		contractID:     uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		billableItemID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	execFixtureSQL(
		t,
		ctx,
		db,
		`INSERT INTO tenants (id, name, currency, timezone, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		ids.tenantID.String(),
		"Test Tenant",
		"USD",
		"UTC",
		now,
	)
	execFixtureSQL(
		t,
		ctx,
		db,
		`INSERT INTO customer_accounts (id, tenant_id, name, status, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		ids.customerID.String(),
		ids.tenantID.String(),
		"Test Customer",
		"active",
		now,
	)
	execFixtureSQL(
		t,
		ctx,
		db,
		`INSERT INTO billable_items (
			id, tenant_id, code, name, unit, pricing_mode, status, created_at
		 )
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ids.billableItemID.String(),
		ids.tenantID.String(),
		"api_calls",
		"API Calls",
		"api_calls",
		"usage",
		"active",
		now,
	)
	execFixtureSQL(
		t,
		ctx,
		db,
		`INSERT INTO contracts (
			id, tenant_id, customer_id, status, start_date, currency,
			version, billing_model, metadata_json, created_at, updated_at
		 )
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ids.contractID.String(),
		ids.tenantID.String(),
		ids.customerID.String(),
		"active",
		time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"USD",
		1,
		"usage_based",
		"{}",
		now,
		now,
	)

	return ids
}

func execFixtureSQL(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	query string,
	args ...any,
) {
	t.Helper()

	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		t.Fatalf("exec fixture sql: %v", err)
	}
}

func sqliteUsageRecord(ids ingestionFixtureIDs) billing.UsageRecord {
	return billing.UsageRecord{
		ID:             uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		TenantID:       ids.tenantID,
		CustomerID:     ids.customerID,
		ContractID:     ids.contractID,
		BillableItemID: ids.billableItemID,
		ExternalID:     "usage-1",
		UsageTime:      time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC),
		Quantity:       100,
		Unit:           "api_calls",
		SourceSystem:   "metering",
		TraceID:        "trace-1",
		Metadata:       map[string]any{},
	}
}

func sqliteInvoice(t *testing.T, ids ingestionFixtureIDs) billing.Invoice {
	t.Helper()

	period, err := valueobject.NewBillingPeriod(
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewBillingPeriod() error = %v", err)
	}

	return billing.Invoice{
		ID:           uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		TenantID:     ids.tenantID,
		CustomerID:   ids.customerID,
		ContractID:   ids.contractID,
		ExternalID:   "invoice-1",
		Number:       "INV-001",
		Period:       period,
		IssuedAt:     time.Date(2026, time.May, 1, 9, 0, 0, 0, time.UTC),
		DueAt:        time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC),
		TotalAmount:  valueobject.MustMoney("USD", 10000),
		Status:       billing.InvoiceStatusIssued,
		SourceSystem: "stripe",
	}
}

func sqliteInvoiceLine(
	invoiceID uuid.UUID,
	billableItemID uuid.UUID,
	lineTotalMinorUnits int64,
) billing.InvoiceLine {
	return billing.InvoiceLine{
		ID:             uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		InvoiceID:      invoiceID,
		BillableItemID: billableItemID,
		Description:    "Platform subscription",
		Quantity:       1,
		UnitPrice:      valueobject.MustMoney("USD", lineTotalMinorUnits),
		DiscountAmount: valueobject.MustMoney("USD", 0),
		TaxAmount:      valueobject.MustMoney("USD", 0),
		LineTotal:      valueobject.MustMoney("USD", lineTotalMinorUnits),
		SourceRef:      "stripe:line-1",
		PricingSnapshot: map[string]any{
			"source": "test",
		},
	}
}
