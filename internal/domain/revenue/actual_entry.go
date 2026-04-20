package revenue

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

type RecognizedFrom string

const (
	RecognizedFromInvoice    RecognizedFrom = "invoice"
	RecognizedFromPayment    RecognizedFrom = "payment"
	RecognizedFromAdjustment RecognizedFrom = "adjustment"
)

type ActualRevenueEntry struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	CustomerID     uuid.UUID
	ContractID     uuid.UUID
	BillableItemID uuid.UUID
	Period         valueobject.BillingPeriod
	ActualAmount   valueobject.Money
	RecognizedFrom RecognizedFrom
	RecognizedAt   time.Time
	TraceID        string
}
