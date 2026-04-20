package payment

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusRefunded  Status = "refunded"
)

type AllocationStatus string

const (
	AllocationStatusUnallocated AllocationStatus = "unallocated"
	AllocationStatusPartial     AllocationStatus = "partial"
	AllocationStatusAllocated   AllocationStatus = "allocated"
)

type Payment struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	CustomerID       uuid.UUID
	ExternalID       string
	Amount           valueobject.Money
	ReceivedAt       time.Time
	Status           Status
	AllocationStatus AllocationStatus
	SourceSystem     string
}
