package reconciliation

import (
	"context"

	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
)

// ListReconciliationRunsCommand aliases the reconciliation service read-model
// command at the application boundary.
type ListReconciliationRunsCommand = reconciliationservice.ListReconciliationRunsCommand

// ListReconciliationRunsResult aliases the reconciliation service read-model
// result at the application boundary.
type ListReconciliationRunsResult = reconciliationservice.ListReconciliationRunsResult

// GetReconciliationRunCommand aliases the reconciliation service detail
// command at the application boundary.
type GetReconciliationRunCommand = reconciliationservice.GetReconciliationRunCommand

// GetReconciliationRunResult aliases the reconciliation service detail result
// at the application boundary.
type GetReconciliationRunResult = reconciliationservice.GetReconciliationRunResult

// ListReconciliationRuns loads recent reconciliation run summaries.
func (w *RevenueLeakageWorkflow) ListReconciliationRuns(
	ctx context.Context,
	cmd ListReconciliationRunsCommand,
) (ListReconciliationRunsResult, error) {
	return w.reconciliation.ListReconciliationRuns(ctx, cmd)
}

// GetReconciliationRun loads one reconciliation run summary.
func (w *RevenueLeakageWorkflow) GetReconciliationRun(
	ctx context.Context,
	cmd GetReconciliationRunCommand,
) (GetReconciliationRunResult, error) {
	return w.reconciliation.GetReconciliationRun(ctx, cmd)
}
