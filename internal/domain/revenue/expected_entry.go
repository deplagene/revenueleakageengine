// Package revenue defines expected and actual revenue ledgers together with the
// resulting revenue differences used by reconciliation.
package revenue

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// ExpectedRevenueEntry records what the platform believes should have been
// earned for a contract and billing period.
type ExpectedRevenueEntry struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	CustomerID       uuid.UUID
	ContractID       uuid.UUID
	BillableItemID   uuid.UUID
	Period           valueobject.BillingPeriod
	ExpectedAmount   valueobject.Money
	CalculationBasis map[string]any
	CalculatedAt     time.Time
	Version          int
	TraceID          string
}
