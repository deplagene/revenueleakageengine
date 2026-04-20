package contract

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusActive    Status = "active"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

type BillingModel string

const (
	BillingModelFixedRecurring BillingModel = "fixed_recurring"
	BillingModelUsageBased     BillingModel = "usage_based"
)

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
