package leakage

import (
	"time"

	"github.com/google/uuid"
)

// StatusHistory records one status transition in the investigation lifecycle.
type StatusHistory struct {
	ID         uuid.UUID
	CaseID     uuid.UUID
	FromStatus Status
	ToStatus   Status
	ChangedAt  time.Time
	ChangedBy  string
	ReasonCode string
	Comment    string
}
