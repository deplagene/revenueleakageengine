package billing

import (
	"time"

	"github.com/google/uuid"
)

// UsageRecord captures billable consumption imported from an external metering
// or product system.
type UsageRecord struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	CustomerID     uuid.UUID
	ContractID     uuid.UUID
	BillableItemID uuid.UUID
	ExternalID     string
	UsageTime      time.Time
	Quantity       int64
	Unit           string
	SourceSystem   string
	TraceID        string
	Metadata       map[string]any
}
