package reconciliation

import (
	"context"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// Store defines the persistence contract needed by the reconciliation service.
// The interface stays in this package because reconciliation owns the use case
// and decides which reads and writes it needs.
type Store interface {
	ListExpectedRevenue(
		ctx context.Context,
		tenantID uuid.UUID,
		contractID uuid.UUID,
		period valueobject.BillingPeriod,
	) ([]revenue.ExpectedRevenueEntry, error)
	ListActualRevenue(
		ctx context.Context,
		tenantID uuid.UUID,
		contractID uuid.UUID,
		period valueobject.BillingPeriod,
	) ([]revenue.ActualRevenueEntry, error)
	CreateReconciliationRun(ctx context.Context, run ReconciliationRun) error
	CompleteReconciliationRun(ctx context.Context, run ReconciliationRun) error
	CreateLeakageCase(ctx context.Context, c leakage.Case, evidence []leakage.Evidence) error
}
