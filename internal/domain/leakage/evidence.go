package leakage

import (
	"time"

	"github.com/google/uuid"
)

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

type Evidence struct {
	ID         uuid.UUID
	CaseID     uuid.UUID
	Type       EvidenceType
	EntityType string
	EntityID   string
	Payload    map[string]any
	CreatedAt  time.Time
}
