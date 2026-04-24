package casework

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	platformsqlite "github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	"github.com/google/uuid"
)

func TestSQLiteStoreListCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestSQLite(t, ctx)
	defer db.Close()

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	seedCaseReferences(t, ctx, db, tenantID, customerID, contractID)
	seedLeakageCase(t, ctx, db, tenantID, customerID, contractID)

	store := NewSQLiteStore(db)
	result, err := store.ListCases(ctx, ListCasesCommand{
		TenantID: tenantID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListCases() error = %v", err)
	}

	if got := len(result); got != 1 {
		t.Fatalf("case count = %d, want 1", got)
	}

	if result[0].Type != "underbilling" {
		t.Fatalf("case type = %s, want underbilling", result[0].Type)
	}

	if result[0].LeakageAmount.MinorUnits != 1160 {
		t.Fatalf("leakage amount = %d, want 1160", result[0].LeakageAmount.MinorUnits)
	}
}

func TestSQLiteStoreGetCase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestSQLite(t, ctx)
	defer db.Close()

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	caseID := uuid.New()
	seedCaseReferences(t, ctx, db, tenantID, customerID, contractID)
	seedLeakageCaseWithID(t, ctx, db, caseID, tenantID, customerID, contractID)
	seedLeakageEvidence(t, ctx, db, caseID)
	seedRootCause(t, ctx, db, caseID)

	store := NewSQLiteStore(db)
	result, err := store.GetCase(ctx, GetCaseCommand{
		TenantID: tenantID,
		CaseID:   caseID,
	})
	if err != nil {
		t.Fatalf("GetCase() error = %v", err)
	}

	if result.Case.ID != caseID {
		t.Fatalf("case id = %s, want %s", result.Case.ID, caseID)
	}

	if got := len(result.Evidence); got != 1 {
		t.Fatalf("evidence count = %d, want 1", got)
	}

	if got := len(result.RootCauses); got != 1 {
		t.Fatalf("root cause count = %d, want 1", got)
	}
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

func seedCaseReferences(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
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
			t.Fatalf("seed case reference: %v", err)
		}
	}
}

func seedLeakageCase(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
) {
	t.Helper()

	seedLeakageCaseWithID(t, ctx, db, uuid.New(), tenantID, customerID, contractID)
}

func seedLeakageCaseWithID(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	caseID uuid.UUID,
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO leakage_cases (
			id,
			tenant_id,
			customer_id,
			contract_id,
			case_type,
			severity,
			status,
			detected_at,
			period_start,
			period_end,
			expected_amount_minor_units,
			actual_amount_minor_units,
			leakage_amount_minor_units,
			currency,
			confidence_score_basis_points,
			root_cause_category,
			assignee,
			trace_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		caseID.String(),
		tenantID.String(),
		customerID.String(),
		contractID.String(),
		"underbilling",
		"high",
		"open",
		"2026-05-01T10:00:00Z",
		"2026-04-01T00:00:00Z",
		"2026-05-01T00:00:00Z",
		int64(5800),
		int64(4640),
		int64(1160),
		"USD",
		int64(9500),
		"",
		"",
		"trace-1",
	)
	if err != nil {
		t.Fatalf("seed leakage case: %v", err)
	}
}

func seedLeakageEvidence(t *testing.T, ctx context.Context, db *sql.DB, caseID uuid.UUID) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO leakage_evidence (
			id,
			case_id,
			evidence_type,
			entity_type,
			entity_id,
			payload_json,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		caseID.String(),
		"pricing_diff",
		"invoice_line",
		"line-1",
		`{"expected_minor_units":5800,"actual_minor_units":4640}`,
		"2026-05-01T10:01:00Z",
	)
	if err != nil {
		t.Fatalf("seed leakage evidence: %v", err)
	}
}

func seedRootCause(t *testing.T, ctx context.Context, db *sql.DB, caseID uuid.UUID) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO root_causes (
			id,
			case_id,
			category,
			subcategory,
			description,
			confidence_score_basis_points,
			derived_by,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		caseID.String(),
		"unauthorized_discount",
		"expired_discount",
		"Discount was still applied after expiry.",
		int64(9000),
		"rules",
		"2026-05-01T10:02:00Z",
	)
	if err != nil {
		t.Fatalf("seed root cause: %v", err)
	}
}
