package leakage

import (
	"time"

	"github.com/google/uuid"
)

// EvidenceType classifies the proof attached to a leakage case.
type EvidenceType string

const (
	EvidenceTypeUsageAggregate    EvidenceType = "usage_aggregate"
	EvidenceTypeInvoiceGap        EvidenceType = "invoice_gap"
	EvidenceTypeContractTermMatch EvidenceType = "contract_term_match"
	EvidenceTypePricingDiff       EvidenceType = "pricing_diff"
	EvidenceTypePaymentDiff       EvidenceType = "payment_diff"
	EvidenceTypeSLADiff           EvidenceType = "sla_diff"
	EvidenceTypeTimelineEvent     EvidenceType = "timeline_event"
)

// Evidence stores the raw or aggregated facts that justify a leakage case.
type Evidence struct {
	ID         uuid.UUID
	CaseID     uuid.UUID
	Type       EvidenceType
	EntityType string
	EntityID   string
	Payload    map[string]any
	CreatedAt  time.Time
}
