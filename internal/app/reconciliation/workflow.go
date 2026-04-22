package reconciliation

import (
	"context"
	"errors"
	"fmt"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	revenueservice "github.com/deplagene/revenueleakageengine/internal/service/revenue"
	"github.com/google/uuid"
)

var (
	// ErrRevenueServiceRequired reports that the workflow has no revenue
	// service dependency.
	ErrRevenueServiceRequired = errors.New("revenue service is required")
	// ErrReconciliationServiceRequired reports that the workflow has no
	// reconciliation service dependency.
	ErrReconciliationServiceRequired = errors.New("reconciliation service is required")
	// ErrWorkflowScopeMismatch reports that expected and actual revenue commands
	// point to different business scopes.
	ErrWorkflowScopeMismatch = errors.New("expected and actual revenue scopes must match")
)

// RevenueLeakageWorkflow coordinates the MVP flow:
// expected revenue -> actual revenue -> reconciliation.
type RevenueLeakageWorkflow struct {
	revenue        *revenueservice.Service
	reconciliation *reconciliationservice.Service
}

// NewRevenueLeakageWorkflow creates an application workflow from business
// services. It does not own storage or transport dependencies directly.
func NewRevenueLeakageWorkflow(
	revenue *revenueservice.Service,
	reconciliation *reconciliationservice.Service,
) (*RevenueLeakageWorkflow, error) {
	if revenue == nil {
		return nil, ErrRevenueServiceRequired
	}

	if reconciliation == nil {
		return nil, ErrReconciliationServiceRequired
	}

	return &RevenueLeakageWorkflow{
		revenue:        revenue,
		reconciliation: reconciliation,
	}, nil
}

// RunRevenueLeakageCheckCommand contains the inputs required for one full MVP
// leakage check.
type RunRevenueLeakageCheckCommand struct {
	Expected             revenueservice.CalculateExpectedRevenueCommand
	Actual               revenueservice.BuildActualRevenueCommand
	RunID                uuid.UUID
	TraceID              string
	Currency             string
	MinimumLeakageAmount valueobject.Money
}

// RunRevenueLeakageCheckResult contains the generated ledger counts and
// reconciliation outcome.
type RunRevenueLeakageCheckResult struct {
	ExpectedEntry        uuid.UUID
	ActualEntryCount     int
	ReconciliationResult reconciliationservice.ReconcilePeriodResult
}

// RunRevenueLeakageCheck calculates expected revenue, builds actual revenue,
// persists both ledgers, and then reconciles them.
func (w *RevenueLeakageWorkflow) RunRevenueLeakageCheck(
	ctx context.Context,
	cmd RunRevenueLeakageCheckCommand,
) (RunRevenueLeakageCheckResult, error) {
	const op = "app.reconciliation.RunRevenueLeakageCheck"

	if err := validateWorkflowScope(cmd.Expected, cmd.Actual); err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: %w", op, err)
	}

	traceID := cmd.TraceID
	if traceID == "" {
		traceID = cmd.Expected.TraceID
	}

	expectedCmd := cmd.Expected
	expectedCmd.TraceID = traceID
	expected, err := w.revenue.CalculateAndSaveExpectedRevenue(ctx, expectedCmd)
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: calculate expected revenue: %w", op, err)
	}

	actualCmd := cmd.Actual
	actualCmd.TraceID = traceID
	actualEntries, err := w.revenue.BuildAndSaveActualRevenue(ctx, actualCmd)
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: build actual revenue: %w", op, err)
	}

	reconciliationResult, err := w.reconciliation.ReconcilePeriod(ctx, reconciliationservice.ReconcilePeriodCommand{
		TenantID:             cmd.Expected.TenantID,
		ContractID:           cmd.Expected.ContractID,
		Period:               cmd.Expected.Period,
		RunID:                runIDOrNew(cmd.RunID),
		TraceID:              traceID,
		Currency:             currencyOrDefault(cmd.Currency, expected.ExpectedAmount.Currency),
		MinimumLeakageAmount: minimumLeakageOrZero(cmd.MinimumLeakageAmount, expected.ExpectedAmount.Currency),
	})
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: reconcile period: %w", op, err)
	}

	return RunRevenueLeakageCheckResult{
		ExpectedEntry:        expected.ID,
		ActualEntryCount:     len(actualEntries),
		ReconciliationResult: reconciliationResult,
	}, nil
}

func validateWorkflowScope(
	expected revenueservice.CalculateExpectedRevenueCommand,
	actual revenueservice.BuildActualRevenueCommand,
) error {
	matchesScope := expected.TenantID == actual.TenantID &&
		expected.CustomerID == actual.CustomerID &&
		expected.ContractID == actual.ContractID &&
		expected.Period.Start.Equal(actual.Period.Start) &&
		expected.Period.End.Equal(actual.Period.End)
	if !matchesScope {
		return ErrWorkflowScopeMismatch
	}

	return nil
}

func runIDOrNew(runID uuid.UUID) uuid.UUID {
	if runID != uuid.Nil {
		return runID
	}

	return uuid.New()
}

func currencyOrDefault(currency, fallback string) string {
	if currency != "" {
		return currency
	}

	return fallback
}

func minimumLeakageOrZero(amount valueobject.Money, currency string) valueobject.Money {
	if amount.Currency != "" {
		return amount
	}

	return valueobject.ZeroMoney(currency)
}
