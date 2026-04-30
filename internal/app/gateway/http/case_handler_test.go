package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type noopReconciliationRunner struct{}

func (n *noopReconciliationRunner) RunRevenueLeakageCheck(
	_ context.Context,
	_ appreconciliation.RunRevenueLeakageCheckCommand,
) (appreconciliation.RunRevenueLeakageCheckResult, error) {
	return appreconciliation.RunRevenueLeakageCheckResult{}, nil
}

func (n *noopReconciliationRunner) ListReconciliationRuns(
	_ context.Context,
	_ appreconciliation.ListReconciliationRunsCommand,
) (appreconciliation.ListReconciliationRunsResult, error) {
	return appreconciliation.ListReconciliationRunsResult{}, nil
}

type recordingCaseQueries struct {
	result       caseapp.ListCasesResult
	err          error
	received     caseapp.ListCasesCommand
	caseResult   caseapp.GetCaseResult
	caseErr      error
	receivedCase caseapp.GetCaseCommand
}

func (q *recordingCaseQueries) ListCases(
	_ context.Context,
	cmd caseapp.ListCasesCommand,
) (caseapp.ListCasesResult, error) {
	q.received = cmd
	return q.result, q.err
}

func (q *recordingCaseQueries) GetCase(
	_ context.Context,
	cmd caseapp.GetCaseCommand,
) (caseapp.GetCaseResult, error) {
	q.receivedCase = cmd
	return q.caseResult, q.caseErr
}

type recordingCaseCommands struct {
	statusResult     caseapp.UpdateCaseStatusResult
	statusErr        error
	receivedStatus   caseapp.UpdateCaseStatusCommand
	assigneeResult   caseapp.UpdateCaseAssigneeResult
	assigneeErr      error
	receivedAssignee caseapp.UpdateCaseAssigneeCommand
}

func (c *recordingCaseCommands) UpdateCaseStatus(
	_ context.Context,
	cmd caseapp.UpdateCaseStatusCommand,
) (caseapp.UpdateCaseStatusResult, error) {
	c.receivedStatus = cmd
	return c.statusResult, c.statusErr
}

func (c *recordingCaseCommands) UpdateCaseAssignee(
	_ context.Context,
	cmd caseapp.UpdateCaseAssigneeCommand,
) (caseapp.UpdateCaseAssigneeResult, error) {
	c.receivedAssignee = cmd
	return c.assigneeResult, c.assigneeErr
}

func TestHandlerListCases(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	contractID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	caseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	caseQueries := &recordingCaseQueries{
		result: caseapp.ListCasesResult{
			Cases: []leakage.Case{
				{
					ID:                caseID,
					TenantID:          tenantID,
					CustomerID:        uuid.MustParse("44444444-4444-4444-4444-444444444444"),
					ContractID:        contractID,
					Type:              leakage.CaseTypeUnderbilling,
					Severity:          leakage.SeverityHigh,
					Status:            leakage.StatusOpen,
					DetectedAt:        time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
					Period:            mustBillingPeriod(t),
					ExpectedAmount:    valueobject.MustMoney("USD", 5800),
					ActualAmount:      valueobject.MustMoney("USD", 4640),
					LeakageAmount:     valueobject.MustMoney("USD", 1160),
					ConfidenceScore:   valueobject.MustConfidenceScore(9500),
					RootCauseCategory: leakage.CategoryUnauthorizedDiscount,
					Assignee:          "billing-team",
					TraceID:           "trace-1",
				},
			},
		},
	}

	handler, err := NewHandler(&noopReconciliationRunner{}, caseQueries, &recordingCaseCommands{}, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/v1/cases?tenant_id="+tenantID.String()+"&contract_id="+contractID.String()+"&status=open&limit=10&offset=5",
		nil,
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if caseQueries.received.TenantID != tenantID {
		t.Fatalf("tenant id = %s, want %s", caseQueries.received.TenantID, tenantID)
	}

	if caseQueries.received.ContractID != contractID {
		t.Fatalf("contract id = %s, want %s", caseQueries.received.ContractID, contractID)
	}

	if caseQueries.received.Status != leakage.StatusOpen {
		t.Fatalf("status = %s, want %s", caseQueries.received.Status, leakage.StatusOpen)
	}

	if caseQueries.received.Limit != 10 {
		t.Fatalf("limit = %d, want 10", caseQueries.received.Limit)
	}

	if caseQueries.received.Offset != 5 {
		t.Fatalf("offset = %d, want 5", caseQueries.received.Offset)
	}

	var payload listCasesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := len(payload.Cases); got != 1 {
		t.Fatalf("case count = %d, want 1", got)
	}

	if payload.Cases[0].ID != caseID.String() {
		t.Fatalf("case id = %s, want %s", payload.Cases[0].ID, caseID)
	}
}

func TestHandlerListCasesBadRequest(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler(&noopReconciliationRunner{}, &recordingCaseQueries{}, &recordingCaseCommands{}, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/v1/cases?tenant_id=bad-uuid",
		nil,
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, stdhttp.StatusBadRequest)
	}
}

func TestHandlerGetCase(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	caseID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	evidenceID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	rootCauseID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	caseQueries := &recordingCaseQueries{
		caseResult: caseapp.GetCaseResult{
			Case: leakage.Case{
				ID:                caseID,
				TenantID:          tenantID,
				CustomerID:        uuid.MustParse("55555555-5555-5555-5555-555555555555"),
				ContractID:        uuid.MustParse("66666666-6666-6666-6666-666666666666"),
				Type:              leakage.CaseTypeUnderbilling,
				Severity:          leakage.SeverityHigh,
				Status:            leakage.StatusOpen,
				DetectedAt:        time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
				Period:            mustBillingPeriod(t),
				ExpectedAmount:    valueobject.MustMoney("USD", 5800),
				ActualAmount:      valueobject.MustMoney("USD", 4640),
				LeakageAmount:     valueobject.MustMoney("USD", 1160),
				ConfidenceScore:   valueobject.MustConfidenceScore(9500),
				RootCauseCategory: leakage.CategoryUnauthorizedDiscount,
				TraceID:           "trace-1",
			},
			Evidence: []leakage.Evidence{
				{
					ID:         evidenceID,
					CaseID:     caseID,
					Type:       leakage.EvidenceTypePricingDiff,
					EntityType: "invoice_line",
					EntityID:   "line-1",
					Payload: map[string]any{
						"expected_minor_units": float64(5800),
						"actual_minor_units":   float64(4640),
					},
					CreatedAt: time.Date(2026, time.May, 1, 10, 1, 0, 0, time.UTC),
				},
			},
			RootCauses: []leakage.RootCause{
				{
					ID:              rootCauseID,
					CaseID:          caseID,
					Category:        leakage.CategoryUnauthorizedDiscount,
					Subcategory:     "expired_discount",
					Description:     "Discount was still applied after expiry.",
					ConfidenceScore: valueobject.MustConfidenceScore(9000),
					DerivedBy:       leakage.DerivedByRules,
					CreatedAt:       time.Date(2026, time.May, 1, 10, 2, 0, 0, time.UTC),
				},
			},
			History: []leakage.StatusHistory{
				{
					ID:         uuid.MustParse("77777777-7777-7777-7777-777777777777"),
					CaseID:     caseID,
					FromStatus: leakage.StatusOpen,
					ToStatus:   leakage.StatusInvestigating,
					ChangedAt:  time.Date(2026, time.May, 1, 10, 3, 0, 0, time.UTC),
					ChangedBy:  "billing-ops",
					ReasonCode: "triage_started",
					Comment:    "Started manual investigation.",
				},
			},
		},
	}

	handler, err := NewHandler(&noopReconciliationRunner{}, caseQueries, &recordingCaseCommands{}, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/v1/cases/"+caseID.String()+"?tenant_id="+tenantID.String(),
		nil,
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if caseQueries.receivedCase.TenantID != tenantID {
		t.Fatalf("tenant id = %s, want %s", caseQueries.receivedCase.TenantID, tenantID)
	}

	if caseQueries.receivedCase.CaseID != caseID {
		t.Fatalf("case id = %s, want %s", caseQueries.receivedCase.CaseID, caseID)
	}

	var payload getCaseResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Case.ID != caseID.String() {
		t.Fatalf("case id = %s, want %s", payload.Case.ID, caseID)
	}

	if got := len(payload.Evidence); got != 1 {
		t.Fatalf("evidence count = %d, want 1", got)
	}

	if got := len(payload.RootCauses); got != 1 {
		t.Fatalf("root cause count = %d, want 1", got)
	}

	if got := len(payload.History); got != 1 {
		t.Fatalf("history count = %d, want 1", got)
	}

	if payload.History[0].ReasonCode != "triage_started" {
		t.Fatalf("history reason code = %q, want %q", payload.History[0].ReasonCode, "triage_started")
	}
}

func TestHandlerGetCaseNotFound(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler(&noopReconciliationRunner{}, &recordingCaseQueries{
		caseErr: caseapp.ErrCaseNotFound,
	}, &recordingCaseCommands{}, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/v1/cases/22222222-2222-2222-2222-222222222222?tenant_id=11111111-1111-1111-1111-111111111111",
		nil,
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusNotFound {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusNotFound, response.Body.String())
	}
}

func TestHandlerPatchCaseStatus(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	caseID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	caseCommands := &recordingCaseCommands{
		statusResult: caseapp.UpdateCaseStatusResult{
			Case: leakage.Case{
				ID:              caseID,
				TenantID:        tenantID,
				CustomerID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
				ContractID:      uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				Type:            leakage.CaseTypeUnderbilling,
				Severity:        leakage.SeverityHigh,
				Status:          leakage.StatusInvestigating,
				DetectedAt:      time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
				Period:          mustBillingPeriod(t),
				ExpectedAmount:  valueobject.MustMoney("USD", 5800),
				ActualAmount:    valueobject.MustMoney("USD", 4640),
				LeakageAmount:   valueobject.MustMoney("USD", 1160),
				ConfidenceScore: valueobject.MustConfidenceScore(9500),
				TraceID:         "trace-1",
			},
		},
	}

	handler, err := NewHandler(&noopReconciliationRunner{}, &recordingCaseQueries{}, caseCommands, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPatch,
		"/api/v1/cases/"+caseID.String()+"/status",
		strings.NewReader(`{"tenant_id":"`+tenantID.String()+`","status":"investigating","changed_by":"billing-ops"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if caseCommands.receivedStatus.TenantID != tenantID {
		t.Fatalf("tenant id = %s, want %s", caseCommands.receivedStatus.TenantID, tenantID)
	}

	if caseCommands.receivedStatus.CaseID != caseID {
		t.Fatalf("case id = %s, want %s", caseCommands.receivedStatus.CaseID, caseID)
	}

	if caseCommands.receivedStatus.Status != leakage.StatusInvestigating {
		t.Fatalf("status = %s, want %s", caseCommands.receivedStatus.Status, leakage.StatusInvestigating)
	}
}

func TestHandlerPatchCaseStatusRejectsResolved(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler(&noopReconciliationRunner{}, &recordingCaseQueries{}, &recordingCaseCommands{}, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPatch,
		"/api/v1/cases/22222222-2222-2222-2222-222222222222/status",
		strings.NewReader(`{"tenant_id":"11111111-1111-1111-1111-111111111111","status":"resolved","changed_by":"billing-ops"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusBadRequest, response.Body.String())
	}
}

func TestHandlerPatchCaseResolve(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	caseID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	caseCommands := &recordingCaseCommands{
		statusResult: caseapp.UpdateCaseStatusResult{
			Case: leakage.Case{
				ID:              caseID,
				TenantID:        tenantID,
				CustomerID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
				ContractID:      uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				Type:            leakage.CaseTypeUnderbilling,
				Severity:        leakage.SeverityHigh,
				Status:          leakage.StatusResolved,
				DetectedAt:      time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
				Period:          mustBillingPeriod(t),
				ExpectedAmount:  valueobject.MustMoney("USD", 5800),
				ActualAmount:    valueobject.MustMoney("USD", 4640),
				LeakageAmount:   valueobject.MustMoney("USD", 1160),
				ConfidenceScore: valueobject.MustConfidenceScore(9500),
				TraceID:         "trace-1",
			},
		},
	}

	handler, err := NewHandler(&noopReconciliationRunner{}, &recordingCaseQueries{}, caseCommands, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPatch,
		"/api/v1/cases/"+caseID.String()+"/resolve",
		strings.NewReader(`{"tenant_id":"`+tenantID.String()+`","reason_code":"invoice_corrected","comment":"Reissued invoice.","changed_by":"billing-ops"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if caseCommands.receivedStatus.Status != leakage.StatusResolved {
		t.Fatalf("status = %s, want %s", caseCommands.receivedStatus.Status, leakage.StatusResolved)
	}

	if caseCommands.receivedStatus.ReasonCode != "invoice_corrected" {
		t.Fatalf("reason code = %q, want %q", caseCommands.receivedStatus.ReasonCode, "invoice_corrected")
	}

	if caseCommands.receivedStatus.Comment != "Reissued invoice." {
		t.Fatalf("comment = %q, want %q", caseCommands.receivedStatus.Comment, "Reissued invoice.")
	}
}

func TestHandlerPatchCaseDismiss(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	caseID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	caseCommands := &recordingCaseCommands{
		statusResult: caseapp.UpdateCaseStatusResult{
			Case: leakage.Case{
				ID:              caseID,
				TenantID:        tenantID,
				CustomerID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
				ContractID:      uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				Type:            leakage.CaseTypeUnderbilling,
				Severity:        leakage.SeverityHigh,
				Status:          leakage.StatusDismissed,
				DetectedAt:      time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
				Period:          mustBillingPeriod(t),
				ExpectedAmount:  valueobject.MustMoney("USD", 5800),
				ActualAmount:    valueobject.MustMoney("USD", 4640),
				LeakageAmount:   valueobject.MustMoney("USD", 1160),
				ConfidenceScore: valueobject.MustConfidenceScore(9500),
				TraceID:         "trace-1",
			},
		},
	}

	handler, err := NewHandler(&noopReconciliationRunner{}, &recordingCaseQueries{}, caseCommands, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPatch,
		"/api/v1/cases/"+caseID.String()+"/dismiss",
		strings.NewReader(`{"tenant_id":"`+tenantID.String()+`","reason_code":"false_positive","comment":"Usage was duplicated by source retry.","changed_by":"billing-ops"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if caseCommands.receivedStatus.Status != leakage.StatusDismissed {
		t.Fatalf("status = %s, want %s", caseCommands.receivedStatus.Status, leakage.StatusDismissed)
	}

	if caseCommands.receivedStatus.ReasonCode != "false_positive" {
		t.Fatalf("reason code = %q, want %q", caseCommands.receivedStatus.ReasonCode, "false_positive")
	}
}

func TestHandlerPatchCaseAssignee(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	caseID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	caseCommands := &recordingCaseCommands{
		assigneeResult: caseapp.UpdateCaseAssigneeResult{
			Case: leakage.Case{
				ID:              caseID,
				TenantID:        tenantID,
				CustomerID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
				ContractID:      uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				Type:            leakage.CaseTypeUnderbilling,
				Severity:        leakage.SeverityHigh,
				Status:          leakage.StatusOpen,
				DetectedAt:      time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
				Period:          mustBillingPeriod(t),
				ExpectedAmount:  valueobject.MustMoney("USD", 5800),
				ActualAmount:    valueobject.MustMoney("USD", 4640),
				LeakageAmount:   valueobject.MustMoney("USD", 1160),
				ConfidenceScore: valueobject.MustConfidenceScore(9500),
				Assignee:        "billing-ops",
				TraceID:         "trace-1",
			},
		},
	}

	handler, err := NewHandler(&noopReconciliationRunner{}, &recordingCaseQueries{}, caseCommands, &noopContractQueries{}, &noopContractCommands{}, &noopIngestionCommands{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPatch,
		"/api/v1/cases/"+caseID.String()+"/assignee",
		strings.NewReader(`{"tenant_id":"`+tenantID.String()+`","assignee":"billing-ops"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if caseCommands.receivedAssignee.TenantID != tenantID {
		t.Fatalf("tenant id = %s, want %s", caseCommands.receivedAssignee.TenantID, tenantID)
	}

	if caseCommands.receivedAssignee.CaseID != caseID {
		t.Fatalf("case id = %s, want %s", caseCommands.receivedAssignee.CaseID, caseID)
	}

	if caseCommands.receivedAssignee.Assignee != "billing-ops" {
		t.Fatalf("assignee = %q, want %q", caseCommands.receivedAssignee.Assignee, "billing-ops")
	}
}

func mustBillingPeriod(t *testing.T) valueobject.BillingPeriod {
	t.Helper()

	start, err := time.Parse(time.RFC3339Nano, "2026-04-01T00:00:00Z")
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}

	end, err := time.Parse(time.RFC3339Nano, "2026-05-01T00:00:00Z")
	if err != nil {
		t.Fatalf("parse end: %v", err)
	}

	period, err := valueobject.NewBillingPeriod(start, end)
	if err != nil {
		t.Fatalf("new billing period: %v", err)
	}

	return period
}
