package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type stubReconciliationRunner struct {
	result   appreconciliation.RunRevenueLeakageCheckResult
	err      error
	received appreconciliation.RunRevenueLeakageCheckCommand
}

func (s *stubReconciliationRunner) RunRevenueLeakageCheck(
	_ context.Context,
	cmd appreconciliation.RunRevenueLeakageCheckCommand,
) (appreconciliation.RunRevenueLeakageCheckResult, error) {
	s.received = cmd
	return s.result, s.err
}

type stubCaseQueries struct {
	result caseapp.ListCasesResult
	err    error
}

func (s *stubCaseQueries) ListCases(
	_ context.Context,
	cmd caseapp.ListCasesCommand,
) (caseapp.ListCasesResult, error) {
	return s.result, s.err
}

func (s *stubCaseQueries) GetCase(
	_ context.Context,
	cmd caseapp.GetCaseCommand,
) (caseapp.GetCaseResult, error) {
	return caseapp.GetCaseResult{}, s.err
}

type stubCaseCommands struct {
	result caseapp.UpdateCaseStatusResult
	err    error
}

func (s *stubCaseCommands) UpdateCaseStatus(
	_ context.Context,
	cmd caseapp.UpdateCaseStatusCommand,
) (caseapp.UpdateCaseStatusResult, error) {
	return s.result, s.err
}

func (s *stubCaseCommands) UpdateCaseAssignee(
	_ context.Context,
	cmd caseapp.UpdateCaseAssigneeCommand,
) (caseapp.UpdateCaseAssigneeResult, error) {
	return caseapp.UpdateCaseAssigneeResult{}, s.err
}

func TestHandlerRunReconciliation(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	customerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	contractID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	billableItemID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	usageID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	invoiceID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	invoiceLineID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	expectedEntryID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	runID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	caseID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	runner := &stubReconciliationRunner{
		result: appreconciliation.RunRevenueLeakageCheckResult{
			ExpectedEntry:    expectedEntryID,
			ActualEntryCount: 1,
			ReconciliationResult: reconciliationservice.ReconcilePeriodResult{
				RunID:         runID,
				DiffCount:     1,
				CaseCount:     1,
				LeakageAmount: valueobject.MustMoney("USD", 1160),
				Cases: []leakage.Case{
					{
						ID:              caseID,
						Type:            leakage.CaseTypeUnderbilling,
						Severity:        leakage.SeverityHigh,
						Status:          leakage.StatusOpen,
						ExpectedAmount:  valueobject.MustMoney("USD", 5800),
						ActualAmount:    valueobject.MustMoney("USD", 4640),
						LeakageAmount:   valueobject.MustMoney("USD", 1160),
						ConfidenceScore: valueobject.MustConfidenceScore(9500),
					},
				},
			},
		},
	}

	handler, err := NewHandler(runner, &stubCaseQueries{}, &stubCaseCommands{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	requestBody := `{
		"tenant_id": "` + tenantID.String() + `",
		"customer_id": "` + customerID.String() + `",
		"contract_id": "` + contractID.String() + `",
		"billable_item_id": "` + billableItemID.String() + `",
		"currency": "USD",
		"trace_id": "trace-001",
		"minimum_leakage_minor_units": 100,
		"period": {
			"start": "2026-04-01T00:00:00Z",
			"end": "2026-05-01T00:00:00Z"
		},
		"pricing": {
			"base_fee_minor_units": 5000,
			"included_quantity": 100000,
			"overage_unit_price_minor_units": 1,
			"unit": "api_calls"
		},
		"usage_records": [
			{
				"id": "` + usageID.String() + `",
				"external_id": "usage-1",
				"usage_time": "2026-04-15T12:00:00Z",
				"quantity": 180000,
				"source_system": "metering"
			}
		],
		"invoices": [
			{
				"id": "` + invoiceID.String() + `",
				"external_id": "invoice-1",
				"number": "INV-001",
				"issued_at": "2026-05-01T09:00:00Z",
				"due_at": "2026-05-15T00:00:00Z",
				"total_amount_minor_units": 4640,
				"status": "issued"
			}
		],
		"invoice_lines": [
			{
				"id": "` + invoiceLineID.String() + `",
				"invoice_id": "` + invoiceID.String() + `",
				"billable_item_id": "` + billableItemID.String() + `",
				"description": "April invoice",
				"quantity": 1,
				"unit_price_minor_units": 4640,
				"discount_amount_minor_units": 0,
				"tax_amount_minor_units": 0,
				"line_total_minor_units": 4640,
				"source_ref": "billing:invoice-line-1"
			}
		]
	}`

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/reconciliation/run",
		strings.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if runner.received.Expected.TenantID != tenantID {
		t.Fatalf("expected tenant id = %s, want %s", runner.received.Expected.TenantID, tenantID)
	}

	if got := len(runner.received.Expected.UsageRecords); got != 1 {
		t.Fatalf("expected usage record count = %d, want 1", got)
	}

	if got := len(runner.received.Actual.Invoices); got != 1 {
		t.Fatalf("expected invoice count = %d, want 1", got)
	}

	var payload runReconciliationResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.RunID != runID.String() {
		t.Fatalf("run id = %s, want %s", payload.RunID, runID)
	}

	if payload.CaseCount != 1 {
		t.Fatalf("case count = %d, want 1", payload.CaseCount)
	}

	if payload.LeakageAmountMinorUnits != 1160 {
		t.Fatalf("leakage amount = %d, want 1160", payload.LeakageAmountMinorUnits)
	}
}

func TestHandlerRunReconciliationBadRequest(t *testing.T) {
	t.Parallel()

	runner := &stubReconciliationRunner{}

	handler, err := NewHandler(runner, &stubCaseQueries{}, &stubCaseCommands{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/reconciliation/run",
		strings.NewReader(`{"tenant_id":"bad-uuid"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, stdhttp.StatusBadRequest)
	}
}
