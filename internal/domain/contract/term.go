package contract

import (
	"time"

	"github.com/google/uuid"
)

type TermType string

const (
	TermTypeBasePrice         TermType = "base_price"
	TermTypeTieredPrice       TermType = "tiered_price"
	TermTypeMinimumCommitment TermType = "minimum_commitment"
	TermTypeUsageCap          TermType = "usage_cap"
	TermTypeDiscount          TermType = "discount"
	TermTypeRebate            TermType = "rebate"
	TermTypePenalty           TermType = "penalty"
	TermTypeSLACredit         TermType = "sla_credit"
	TermTypePaymentTerms      TermType = "payment_terms"
	TermTypeRenewalRule       TermType = "renewal_rule"
	TermTypeInvoiceSchedule   TermType = "invoice_schedule"
)

type Term struct {
	ID            uuid.UUID
	ContractID    uuid.UUID
	Type          TermType
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
	Priority      int
	Expression    map[string]any
	SourceRef     string
}
