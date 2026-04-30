package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestUIDashboardRendersRecentRuns(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	contractID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	runID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	handler, err := NewHandler(
		&stubReconciliationRunner{
			runs: appreconciliation.ListReconciliationRunsResult{
				Runs: []reconciliationservice.ReconciliationRunSummary{
					{
						ID:            runID,
						TenantID:      tenantID,
						ContractID:    contractID,
						Period:        mustUIBillingPeriod(t),
						Status:        reconciliationservice.ReconciliationRunStatusCompleted,
						StartedAt:     time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC),
						ExpectedCount: 1,
						ActualCount:   1,
						DiffCount:     1,
						CaseCount:     1,
						LeakageAmount: valueobject.MustMoney("USD", 25_00),
					},
				},
			},
		},
		&recordingCaseQueries{},
		&recordingCaseCommands{},
		&noopContractQueries{},
		&noopContractCommands{},
		&noopIngestionCommands{},
	)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ui?tenant_id="+tenantID.String(), nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}

	body := response.Body.String()
	if !strings.Contains(body, "Операционная консоль выручки") {
		t.Fatalf("body does not contain dashboard title: %s", body)
	}

	if !strings.Contains(body, runID.String()) {
		t.Fatalf("body does not contain run id: %s", body)
	}
}

func TestUIRunReconciliationFormCallsRunner(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	contractID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	runID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	runner := &stubReconciliationRunner{
		result: appreconciliation.RunRevenueLeakageCheckResult{
			ExpectedEntry: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			ReconciliationResult: reconciliationservice.ReconcilePeriodResult{
				RunID:         runID,
				ActualCount:   1,
				DiffCount:     1,
				CaseCount:     0,
				LeakageAmount: valueobject.MustMoney("USD", 0),
			},
		},
	}
	handler, err := NewHandler(
		runner,
		&recordingCaseQueries{},
		&recordingCaseCommands{},
		&noopContractQueries{},
		&noopContractCommands{},
		&noopIngestionCommands{},
	)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	form := strings.NewReader(
		"tenant_id=" + tenantID.String() +
			"&contract_id=" + contractID.String() +
			"&period_start=2026-04-01T00:00:00Z" +
			"&period_end=2026-05-01T00:00:00Z" +
			"&currency=USD" +
			"&minimum_leakage_minor_units=0" +
			"&trace_id=ui-test",
	)
	request := httptest.NewRequest(http.MethodPost, "/ui/reconciliation/run", form)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}

	if runner.received.TenantID != tenantID {
		t.Fatalf("tenant id = %s, want %s", runner.received.TenantID, tenantID)
	}

	if !strings.Contains(response.Body.String(), runID.String()) {
		t.Fatalf("body does not contain run id: %s", response.Body.String())
	}
}

func TestUICasesHTMXRendersCasesTable(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	caseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	handler, err := NewHandler(
		&noopReconciliationRunner{},
		&recordingCaseQueries{
			result: caseapp.ListCasesResult{
				Cases: []leakage.Case{
					{
						ID:              caseID,
						TenantID:        tenantID,
						ContractID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
						Type:            leakage.CaseTypeUnderbilling,
						Severity:        leakage.SeverityMedium,
						Status:          leakage.StatusOpen,
						DetectedAt:      time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC),
						Period:          mustUIBillingPeriod(t),
						ExpectedAmount:  valueobject.MustMoney("USD", 100_00),
						ActualAmount:    valueobject.MustMoney("USD", 75_00),
						LeakageAmount:   valueobject.MustMoney("USD", 25_00),
						ConfidenceScore: valueobject.MustConfidenceScore(10_000),
					},
				},
			},
		},
		&recordingCaseCommands{},
		&noopContractQueries{},
		&noopContractCommands{},
		&noopIngestionCommands{},
	)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/ui/cases?tenant_id="+tenantID.String(), nil)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}

	body := response.Body.String()
	if !strings.Contains(body, `id="cases-table"`) {
		t.Fatalf("body does not contain cases table: %s", body)
	}

	if !strings.Contains(body, caseID.String()) {
		t.Fatalf("body does not contain case id: %s", body)
	}
}

func mustUIBillingPeriod(t *testing.T) valueobject.BillingPeriod {
	t.Helper()

	period, err := valueobject.NewBillingPeriod(
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewBillingPeriod() error = %v", err)
	}

	return period
}
