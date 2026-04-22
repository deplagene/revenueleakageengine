package billing

import (
	"time"

	"github.com/google/uuid"
)

// SLAEvent describes an outage or service-quality event that may trigger a
// credit, penalty, or investigation.
type SLAEvent struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	CustomerID  uuid.UUID
	ContractID  uuid.UUID
	EventType   string
	StartedAt   time.Time
	EndedAt     time.Time
	Severity    string
	ImpactScope string
	SourceRef   string
}
