package grpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	revenueleakageenginev1 "github.com/deplagene/revenueleakageengine/internal/gen/proto/revenueleakageengine/v1"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHandlerRunMapsRequestAndResponse(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	contractID := uuid.New()
	runID := uuid.New()
	expectedID := uuid.New()
	caseID := uuid.New()
	periodStart := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)

	reconciliation := &fakeReconciliationRunner{
		runResult: appreconciliation.RunRevenueLeakageCheckResult{
			ExpectedEntry:    expectedID,
			ActualEntryCount: 2,
			ReconciliationResult: reconciliationservice.ReconcilePeriodResult{
				RunID:         runID,
				DiffCount:     3,
				CaseCount:     1,
				LeakageAmount: valueobject.MustMoney("USD", 116_000),
				Cases: []leakage.Case{
					{
						ID:            caseID,
						TenantID:      tenantID,
						ContractID:    contractID,
						Severity:      leakage.SeverityHigh,
						Status:        leakage.StatusOpen,
						DetectedAt:    periodEnd,
						Period:        valueobject.BillingPeriod{Start: periodStart, End: periodEnd},
						LeakageAmount: valueobject.MustMoney("USD", 116_000),
					},
				},
			},
		},
	}
	handler, err := NewHandler(reconciliation, &fakeCaseQueries{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	response, err := handler.Run(context.Background(), &revenueleakageenginev1.RunRequest{
		TenantId:                 tenantID.String(),
		ContractId:               contractID.String(),
		Period:                   &revenueleakageenginev1.BillingPeriod{Start: timestamppb.New(periodStart), End: timestamppb.New(periodEnd)},
		Currency:                 "USD",
		MinimumLeakageMinorUnits: 100,
		RunId:                    runID.String(),
		TraceId:                  "trace-grpc",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if reconciliation.runCmd.TenantID != tenantID {
		t.Fatalf("TenantID = %s, want %s", reconciliation.runCmd.TenantID, tenantID)
	}
	if reconciliation.runCmd.ContractID != contractID {
		t.Fatalf("ContractID = %s, want %s", reconciliation.runCmd.ContractID, contractID)
	}
	if reconciliation.runCmd.MinimumLeakageAmount.MinorUnits != 100 {
		t.Fatalf("MinimumLeakageAmount = %d, want 100", reconciliation.runCmd.MinimumLeakageAmount.MinorUnits)
	}
	if response.GetRunId() != runID.String() {
		t.Fatalf("RunId = %s, want %s", response.GetRunId(), runID)
	}
	if response.GetExpectedEntryId() != expectedID.String() {
		t.Fatalf("ExpectedEntryId = %s, want %s", response.GetExpectedEntryId(), expectedID)
	}
	if response.GetCases()[0].GetStatus() != revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_OPEN {
		t.Fatalf("case status = %s, want open", response.GetCases()[0].GetStatus())
	}
}

func TestHandlerGetRunMapsNotFound(t *testing.T) {
	t.Parallel()

	reconciliation := &fakeReconciliationRunner{
		getRunErr: fmt.Errorf("wrapped: %w", reconciliationservice.ErrRunNotFound),
	}
	handler, err := NewHandler(reconciliation, &fakeCaseQueries{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	_, err = handler.GetRun(context.Background(), &revenueleakageenginev1.GetRunRequest{
		TenantId: uuid.New().String(),
		RunId:    uuid.New().String(),
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetRun() code = %s, want %s", status.Code(err), codes.NotFound)
	}
}

func TestHandlerListCasesRejectsInvalidTenant(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler(&fakeReconciliationRunner{}, &fakeCaseQueries{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	_, err = handler.ListCases(context.Background(), &revenueleakageenginev1.ListCasesRequest{
		TenantId: "not-a-uuid",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("ListCases() code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

type fakeReconciliationRunner struct {
	runCmd    appreconciliation.RunRevenueLeakageCheckCommand
	runResult appreconciliation.RunRevenueLeakageCheckResult
	runErr    error

	getRunResult appreconciliation.GetReconciliationRunResult
	getRunErr    error
}

func (f *fakeReconciliationRunner) RunRevenueLeakageCheck(
	_ context.Context,
	cmd appreconciliation.RunRevenueLeakageCheckCommand,
) (appreconciliation.RunRevenueLeakageCheckResult, error) {
	f.runCmd = cmd
	return f.runResult, f.runErr
}

func (f *fakeReconciliationRunner) ListReconciliationRuns(
	_ context.Context,
	_ appreconciliation.ListReconciliationRunsCommand,
) (appreconciliation.ListReconciliationRunsResult, error) {
	return appreconciliation.ListReconciliationRunsResult{}, nil
}

func (f *fakeReconciliationRunner) GetReconciliationRun(
	_ context.Context,
	_ appreconciliation.GetReconciliationRunCommand,
) (appreconciliation.GetReconciliationRunResult, error) {
	return f.getRunResult, f.getRunErr
}

type fakeCaseQueries struct {
	result caseapp.ListCasesResult
	err    error
}

func (f *fakeCaseQueries) ListCases(
	_ context.Context,
	_ caseapp.ListCasesCommand,
) (caseapp.ListCasesResult, error) {
	return f.result, f.err
}
