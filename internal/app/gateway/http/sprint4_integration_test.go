//go:build integration

package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	contractapp "github.com/deplagene/revenueleakageengine/internal/app/contract"
	ingestionapp "github.com/deplagene/revenueleakageengine/internal/app/ingestion"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	platformsqlite "github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	casework "github.com/deplagene/revenueleakageengine/internal/service/case"
	contractwork "github.com/deplagene/revenueleakageengine/internal/service/contract"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	revenueservice "github.com/deplagene/revenueleakageengine/internal/service/revenue"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestSprint4SmokeDatabaseDrivenReconciliation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openSprint4SmokeSQLite(t, ctx)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	customerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	contractID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	billableItemID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	seedSprint4TenantAndCustomer(t, ctx, db, tenantID, customerID)

	router := newSprint4SmokeRouter(t, db)

	postSprint4JSON[map[string]any](t, router, "/api/v1/contracts/billable-items", map[string]any{
		"id":           billableItemID.String(),
		"tenant_id":    tenantID.String(),
		"code":         "platform_subscription",
		"name":         "Platform subscription",
		"category":     "platform",
		"unit":         "events",
		"pricing_mode": "usage",
		"status":       "active",
	}, stdhttp.StatusCreated)

	postSprint4JSON[map[string]any](t, router, "/api/v1/contracts", map[string]any{
		"id":            contractID.String(),
		"tenant_id":     tenantID.String(),
		"customer_id":   customerID.String(),
		"external_id":   "contract-acme-2026",
		"status":        "active",
		"start_date":    "2026-04-01T00:00:00Z",
		"currency":      "USD",
		"version":       1,
		"billing_model": "usage_based",
	}, stdhttp.StatusCreated)

	postSprint4JSON[map[string]any](t, router, "/api/v1/contracts/"+contractID.String()+"/terms", map[string]any{
		"type":           "fixed_fee",
		"code":           "platform_subscription",
		"amount":         500_000,
		"currency":       "USD",
		"effective_from": "2026-04-01T00:00:00Z",
		"priority":       1,
	}, stdhttp.StatusCreated)

	postSprint4JSON[map[string]any](t, router, "/api/v1/contracts/"+contractID.String()+"/terms", map[string]any{
		"type":     "usage_rate",
		"code":     "platform_subscription",
		"amount":   1,
		"currency": "USD",
		"expression": map[string]any{
			"included_quantity": 100_000,
			"unit":              "events",
		},
		"effective_from": "2026-04-01T00:00:00Z",
		"priority":       2,
	}, stdhttp.StatusCreated)

	postSprint4JSON[map[string]any](t, router, "/api/v1/ingest/usage", map[string]any{
		"records": []map[string]any{
			{
				"tenant_id":        tenantID.String(),
				"customer_id":      customerID.String(),
				"contract_id":      contractID.String(),
				"billable_item_id": billableItemID.String(),
				"external_id":      "usage-april-001",
				"usage_time":       "2026-04-10T12:00:00Z",
				"quantity":         180_000,
				"unit":             "events",
				"source_system":    "metering",
				"trace_id":         "trace-sprint-4",
			},
		},
	}, stdhttp.StatusOK)

	postSprint4JSON[map[string]any](t, router, "/api/v1/ingest/invoices", map[string]any{
		"tenant_id":                tenantID.String(),
		"customer_id":              customerID.String(),
		"contract_id":              contractID.String(),
		"external_id":              "stripe-invoice-april",
		"number":                   "INV-2026-04",
		"period_start":             "2026-04-01T00:00:00Z",
		"period_end":               "2026-05-01T00:00:00Z",
		"issued_at":                "2026-05-01T09:00:00Z",
		"due_at":                   "2026-05-15T00:00:00Z",
		"currency":                 "USD",
		"total_amount_minor_units": 464_000,
		"status":                   "issued",
		"source_system":            "stripe",
		"lines": []map[string]any{
			{
				"billable_item_id":            billableItemID.String(),
				"description":                 "Platform subscription and usage",
				"quantity":                    1,
				"unit_price_minor_units":      464_000,
				"discount_amount_minor_units": 0,
				"tax_amount_minor_units":      0,
				"line_total_minor_units":      464_000,
				"source_ref":                  "stripe:line-april-001",
			},
		},
	}, stdhttp.StatusOK)

	runResponse := postSprint4JSON[runReconciliationResponse](t, router, "/api/v1/reconciliation/run", map[string]any{
		"tenant_id":   tenantID.String(),
		"contract_id": contractID.String(),
		"trace_id":    "trace-sprint-4",
		"period": map[string]any{
			"start": "2026-04-01T00:00:00Z",
			"end":   "2026-05-01T00:00:00Z",
		},
	}, stdhttp.StatusOK)

	if runResponse.ActualEntryCount != 1 {
		t.Fatalf("actual entry count = %d, want 1", runResponse.ActualEntryCount)
	}

	if runResponse.CaseCount != 1 {
		t.Fatalf("case count = %d, want 1", runResponse.CaseCount)
	}

	if runResponse.LeakageAmountMinorUnits != 116_000 {
		t.Fatalf("leakage amount = %d, want 116000", runResponse.LeakageAmountMinorUnits)
	}

	assertSprint4PersistedRevenueAndCase(t, ctx, db, runResponse, tenantID, contractID)

	casesResponse := getSprint4JSON[listCasesResponse](
		t,
		router,
		"/api/v1/cases?tenant_id="+tenantID.String(),
		stdhttp.StatusOK,
	)
	if len(casesResponse.Cases) != 1 {
		t.Fatalf("cases response count = %d, want 1", len(casesResponse.Cases))
	}

	if casesResponse.Cases[0].ReconciliationRunID != runResponse.RunID {
		t.Fatalf(
			"case response run id = %s, want %s",
			casesResponse.Cases[0].ReconciliationRunID,
			runResponse.RunID,
		)
	}
}

func newSprint4SmokeRouter(t *testing.T, db *sql.DB) stdhttp.Handler {
	t.Helper()

	contractStore := contractwork.NewSQLiteStore(db)
	contractService, err := contractwork.NewService(contractStore)
	if err != nil {
		t.Fatalf("build contract service: %v", err)
	}

	contractQueries := contractapp.NewQueries(contractService)
	contractCommands := contractapp.NewCommands(contractService)

	ingestionStore := ingestionapp.NewSQLiteStore(db)
	ingestionCommands, err := ingestionapp.NewCommands(ingestionStore)
	if err != nil {
		t.Fatalf("build ingestion commands: %v", err)
	}

	ingestionQueries, err := ingestionapp.NewQueries(ingestionStore)
	if err != nil {
		t.Fatalf("build ingestion queries: %v", err)
	}

	revenueStore := revenueservice.NewSQLiteStore(db)
	revenueService := revenueservice.NewService(revenueservice.WithStore(revenueStore))

	reconciliationStore := reconciliationservice.NewSQLiteStore(db)
	reconciliationService, err := reconciliationservice.NewService(reconciliationStore)
	if err != nil {
		t.Fatalf("build reconciliation service: %v", err)
	}

	reconciliationWorkflow, err := appreconciliation.NewRevenueLeakageWorkflow(
		revenueService,
		reconciliationService,
		contractQueries,
		ingestionQueries,
	)
	if err != nil {
		t.Fatalf("build reconciliation workflow: %v", err)
	}

	caseStore := casework.NewSQLiteStore(db)
	caseService, err := casework.NewService(caseStore)
	if err != nil {
		t.Fatalf("build case service: %v", err)
	}

	caseQueries, err := caseapp.NewQueries(caseService)
	if err != nil {
		t.Fatalf("build case queries: %v", err)
	}

	caseCommands, err := caseapp.NewCommands(caseService)
	if err != nil {
		t.Fatalf("build case commands: %v", err)
	}

	handler, err := NewHandler(
		reconciliationWorkflow,
		caseQueries,
		caseCommands,
		contractQueries,
		contractCommands,
		ingestionCommands,
	)
	if err != nil {
		t.Fatalf("build http handler: %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	return router
}

func openSprint4SmokeSQLite(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	db, err := platformsqlite.Open(
		ctx,
		"file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared&_pragma=foreign_keys(1)",
	)
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}

	schema, err := os.ReadFile(sprint4SQLiteSchemaPath(t))
	if err != nil {
		t.Fatalf("read sqlite schema: %v", err)
	}

	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		t.Fatalf("apply sqlite schema: %v", err)
	}

	return db
}

func sprint4SQLiteSchemaPath(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file path")
	}

	return filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"..",
		"platform",
		"sqlite",
		"schema",
		"schema.sql",
	)
}

func seedSprint4TenantAndCustomer(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	tenantID uuid.UUID,
	customerID uuid.UUID,
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
			query: `INSERT INTO customer_accounts (id, tenant_id, external_id, name, status, created_at)
				VALUES (?, ?, ?, ?, ?, ?)`,
			args: []any{
				customerID.String(),
				tenantID.String(),
				"acme-customer",
				"Acme Account",
				"active",
				now,
			},
		},
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed tenant/customer: %v", err)
		}
	}
}

func postSprint4JSON[T any](
	t *testing.T,
	handler stdhttp.Handler,
	path string,
	body map[string]any,
	wantStatus int,
) T {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	request := httptest.NewRequest(stdhttp.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertSprint4Status(t, response, wantStatus)

	var decoded T
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response body: %v; body=%s", err, response.Body.String())
	}

	return decoded
}

func getSprint4JSON[T any](
	t *testing.T,
	handler stdhttp.Handler,
	path string,
	wantStatus int,
) T {
	t.Helper()

	request := httptest.NewRequest(stdhttp.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertSprint4Status(t, response, wantStatus)

	var decoded T
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response body: %v; body=%s", err, response.Body.String())
	}

	return decoded
}

func assertSprint4Status(t *testing.T, response *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()

	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", response.Code, wantStatus, response.Body.String())
	}
}

func assertSprint4PersistedRevenueAndCase(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	response runReconciliationResponse,
	tenantID uuid.UUID,
	contractID uuid.UUID,
) {
	t.Helper()

	runID, err := uuid.Parse(response.RunID)
	if err != nil {
		t.Fatalf("parse run id: %v", err)
	}

	var run reconciliationRunSnapshot
	err = db.QueryRowContext(
		ctx,
		`SELECT status, expected_count, actual_count, diff_count, case_count, leakage_amount_minor_units
			FROM reconciliation_runs
			WHERE id = ? AND tenant_id = ? AND contract_id = ?`,
		runID.String(),
		tenantID.String(),
		contractID.String(),
	).Scan(
		&run.Status,
		&run.ExpectedCount,
		&run.ActualCount,
		&run.DiffCount,
		&run.CaseCount,
		&run.LeakageAmountMinorUnits,
	)
	if err != nil {
		t.Fatalf("query reconciliation run: %v", err)
	}

	if run != (reconciliationRunSnapshot{
		Status:                  "completed",
		ExpectedCount:           1,
		ActualCount:             1,
		DiffCount:               1,
		CaseCount:               1,
		LeakageAmountMinorUnits: 116_000,
	}) {
		t.Fatalf("run snapshot = %+v, want completed 1/1/1/1 leakage 116000", run)
	}

	assertSprint4SingleAmount(t, ctx, db, "expected_revenue_entries", "expected_amount_minor_units", 580_000)
	assertSprint4SingleAmount(t, ctx, db, "actual_revenue_entries", "actual_amount_minor_units", 464_000)

	var c leakageCaseSnapshot
	err = db.QueryRowContext(
		ctx,
		`SELECT reconciliation_run_id, case_type, status, leakage_amount_minor_units
			FROM leakage_cases
			WHERE tenant_id = ? AND contract_id = ?`,
		tenantID.String(),
		contractID.String(),
	).Scan(&c.ReconciliationRunID, &c.CaseType, &c.Status, &c.LeakageAmountMinorUnits)
	if err != nil {
		t.Fatalf("query leakage case: %v", err)
	}

	if c != (leakageCaseSnapshot{
		ReconciliationRunID:     runID.String(),
		CaseType:                "underbilling",
		Status:                  "open",
		LeakageAmountMinorUnits: 116_000,
	}) {
		t.Fatalf("case snapshot = %+v, want linked underbilling open leakage 116000", c)
	}
}

func assertSprint4SingleAmount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	table string,
	column string,
	want int64,
) {
	t.Helper()

	var got int64
	query := fmt.Sprintf("SELECT %s FROM %s", column, table)
	if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil {
		t.Fatalf("query %s.%s: %v", table, column, err)
	}

	if got != want {
		t.Fatalf("%s.%s = %d, want %d", table, column, got, want)
	}
}

type reconciliationRunSnapshot struct {
	Status                  string
	ExpectedCount           int
	ActualCount             int
	DiffCount               int
	CaseCount               int
	LeakageAmountMinorUnits int64
}

type leakageCaseSnapshot struct {
	ReconciliationRunID     string
	CaseType                string
	Status                  string
	LeakageAmountMinorUnits int64
}
