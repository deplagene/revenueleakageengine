// Package contract defines commercial truth such as contracts, billable items,
// and contract terms used to calculate expected revenue.
package contract

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle state of a contract.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusActive    Status = "active"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

// BillingModel identifies the pricing pattern used by a contract.
type BillingModel string

const (
	BillingModelFixedRecurring BillingModel = "fixed_recurring"
	BillingModelUsageBased     BillingModel = "usage_based"
)

// Contract stores the commercial agreement that defines how revenue should be
// earned from a customer account.
type Contract struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	CustomerID   uuid.UUID
	ExternalID   string
	Status       Status
	StartDate    time.Time
	EndDate      *time.Time
	Currency     string
	Version      int
	BillingModel BillingModel
	SignedAt     *time.Time
	Metadata     map[string]any
}
