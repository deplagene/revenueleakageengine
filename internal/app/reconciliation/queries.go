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

// ListReconciliationRuns loads recent reconciliation run summaries.
func (w *RevenueLeakageWorkflow) ListReconciliationRuns(
	ctx context.Context,
	cmd ListReconciliationRunsCommand,
) (ListReconciliationRunsResult, error) {
	return w.reconciliation.ListReconciliationRuns(ctx, cmd)
}
