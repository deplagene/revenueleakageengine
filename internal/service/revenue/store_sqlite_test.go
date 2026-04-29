package revenue

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	platformsqlite "github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	"github.com/google/uuid"
)

func TestSQLiteStoreSaveExpectedRevenue(t *testing.T) {
	ctx := context.Background()
	db := openTestSQLite(t, ctx)

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	seedExpectedRevenueReferences(
		t,
		ctx,
		db,
		tenantID,
		customerID,
		contractID,
		billableItemID,
	)

	store := NewSQLiteStore(db)
	period := mustPeriod(t)
	entry := revenuedomain.ExpectedRevenueEntry{
		ID:             uuid.New(),
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		Period:         period,
		ExpectedAmount: valueobject.MustMoney("USD", 580_000),
		CalculationBasis: map[string]any{
			"model":          "fixed_usage",
			"usage_quantity": int64(180_000),
		},
		CalculatedAt: mustTime(t, "2026-05-01T01:00:00Z"),
		Version:      1,
		TraceID:      "trace-1",
	}

	if err := store.SaveExpectedRevenue(ctx, entry); err != nil {
		t.Fatalf("SaveExpectedRevenue() error = %v", err)
	}

	var amount int64
	var basis string
	err := db.QueryRowContext(
		ctx,
		`SELECT expected_amount_minor_units, calculation_basis_json
		 FROM expected_revenue_entries
		 WHERE id = ?`,
		entry.ID.String(),
	).Scan(&amount, &basis)
	if err != nil {
		t.Fatalf("query expected revenue entry: %v", err)
	}

	if amount != 580_000 {
		t.Fatalf("amount = %d, want 580000", amount)
	}

	if !strings.Contains(basis, `"model":"fixed_usage"`) {
		t.Fatalf("calculation basis = %s, want fixed_usage model", basis)
	}
}

func TestSQLiteStoreSaveActualRevenue(t *testing.T) {
	ctx := context.Background()
	db := openTestSQLite(t, ctx)

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	seedExpectedRevenueReferences(
		t,
		ctx,
		db,
		tenantID,
		customerID,
		contractID,
		billableItemID,
	)

	store := NewSQLiteStore(db)
	period := mustPeriod(t)
	entry := revenuedomain.ActualRevenueEntry{
		ID:             uuid.New(),
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		Period:         period,
		ActualAmount:   valueobject.MustMoney("USD", 464_000),
		RecognizedFrom: revenuedomain.RecognizedFromInvoice,
		RecognizedAt:   mustTime(t, "2026-04-30T23:00:00Z"),
		TraceID:        "trace-actual-1",
	}

	if err := store.SaveActualRevenue(ctx, entry); err != nil {
		t.Fatalf("SaveActualRevenue() error = %v", err)
	}

	var amount int64
	var recognizedFrom string
	err := db.QueryRowContext(
		ctx,
		`SELECT actual_amount_minor_units, recognized_from
		 FROM actual_revenue_entries
		 WHERE id = ?`,
		entry.ID.String(),
	).Scan(&amount, &recognizedFrom)
	if err != nil {
		t.Fatalf("query actual revenue entry: %v", err)
	}

	if amount != 464_000 {
		t.Fatalf("amount = %d, want 464000", amount)
	}

	if recognizedFrom != "invoice" {
		t.Fatalf("recognized_from = %q, want invoice", recognizedFrom)
	}
}

func openTestSQLite(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	db, err := platformsqlite.Open(
		ctx,
		"file:revenue_store_test?mode=memory&cache=shared&_pragma=foreign_keys(1)",
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

func seedExpectedRevenueReferences(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	billableItemID uuid.UUID,
) {
	t.Helper()

	now := formatStoredTime(time.Now())
	statements := []struct {
		query string
		args  []any
	}{
		{
			query: `INSERT INTO tenants (id, name, currency, timezone, created_at)
				VALUES (?, ?, ?, ?, ?)`,
			args: []any{tenantID.String(), "Acme", "USD", "UTC", now},
		},
		{
			query: `INSERT INTO customer_accounts (id, tenant_id, name, status, created_at)
				VALUES (?, ?, ?, ?, ?)`,
			args: []any{customerID.String(), tenantID.String(), "Acme Account", "active", now},
		},
		{
			query: `INSERT INTO billable_items (id, tenant_id, code, name, unit, pricing_mode, status, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			args: []any{
				billableItemID.String(),
				tenantID.String(),
				"events",
				"Events",
				"events",
				"usage",
				"active",
				now,
			},
		},
		{
			query: `INSERT INTO contracts (
					id,
					tenant_id,
					customer_id,
					status,
					start_date,
					currency,
					version,
					billing_model,
					created_at,
					updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			args: []any{
				contractID.String(),
				tenantID.String(),
				customerID.String(),
				"active",
				"2026-04-01T00:00:00Z",
				"USD",
				int64(1),
				"usage_based",
				now,
				now,
			},
		},
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed expected revenue reference: %v", err)
		}
	}
}
