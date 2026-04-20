// Package payment defines payment-side facts such as incoming payments and
// their allocations to invoices.
package payment

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// Status represents the processing state of a payment.
type Status string

const (
	StatusPending   Status = "pending"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusRefunded  Status = "refunded"
)

// AllocationStatus describes how fully a payment has been distributed across
// invoices.
type AllocationStatus string

const (
	AllocationStatusUnallocated AllocationStatus = "unallocated"
	AllocationStatusPartial     AllocationStatus = "partial"
	AllocationStatusAllocated   AllocationStatus = "allocated"
)

// Payment records money received from a customer before or after invoice
// allocation.
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
