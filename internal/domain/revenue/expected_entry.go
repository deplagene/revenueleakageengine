package revenue

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

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
