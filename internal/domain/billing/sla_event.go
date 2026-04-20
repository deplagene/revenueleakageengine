package billing

import (
	"time"

	"github.com/google/uuid"
)

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
