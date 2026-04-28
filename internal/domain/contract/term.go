package contract

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrTermIDRequired reports that a persisted term is missing its identifier.
	ErrTermIDRequired = errors.New("term id is required")
	// ErrTermTypeRequired reports that a contract term lacks its rule type.
	ErrTermTypeRequired = errors.New("term type is required")
	// ErrTermTypeInvalid reports an unknown contract term type.
	ErrTermTypeInvalid = errors.New("term type is invalid")
	// ErrEffectiveFromRequired reports that a term lacks the timestamp from
	// which it becomes valid.
	ErrEffectiveFromRequired = errors.New("term effective_from is required")
	// ErrEffectiveRangeInvalid reports a term effective_to value that does not
	// follow effective_from.
	ErrEffectiveRangeInvalid = errors.New("term effective_to must be after effective_from")
	// ErrTermExpressionRequired reports that the term does not include the
	// auditable pricing expression needed for revenue calculation.
	ErrTermExpressionRequired = errors.New("term expression is required")
)

// TermType classifies the rule encoded by a contract term.
type TermType string

const (
	TermTypeFixedFee          TermType = "fixed_fee"
	TermTypeUsageRate         TermType = "usage_rate"
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

// Term stores one effective contract rule such as a price, discount, or SLA
// credit condition.
type Term struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	ContractID    uuid.UUID
	Type          TermType
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
	Priority      int
	Expression    map[string]any
	SourceRef     string
}

// Normalize returns a copy with UTC timestamps and non-nil expression map.
func (t Term) Normalize() Term {
	if t.EffectiveFrom.Location() != time.UTC {
		t.EffectiveFrom = t.EffectiveFrom.UTC()
	}

	if t.EffectiveTo != nil {
		effectiveTo := t.EffectiveTo.UTC()
		t.EffectiveTo = &effectiveTo
	}

	if t.Expression == nil {
		t.Expression = map[string]any{}
	}

	return t
}

// Validate checks the invariants required before a term becomes part of the
// effective contract pricing timeline.
func (t Term) Validate() error {
	if t.ID == uuid.Nil {
		return ErrTermIDRequired
	}

	if t.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if t.ContractID == uuid.Nil {
		return ErrContractIDRequired
	}

	if err := t.Type.Validate(); err != nil {
		return err
	}

	if t.EffectiveFrom.IsZero() {
		return ErrEffectiveFromRequired
	}

	if t.EffectiveTo != nil && !t.EffectiveTo.After(t.EffectiveFrom) {
		return ErrEffectiveRangeInvalid
	}

	if len(t.Expression) == 0 {
		return ErrTermExpressionRequired
	}

	return nil
}

// Validate checks that the term type is supported.
func (t TermType) Validate() error {
	switch t {
	case "":
		return ErrTermTypeRequired
	case TermTypeFixedFee,
		TermTypeUsageRate,
		TermTypeBasePrice,
		TermTypeTieredPrice,
		TermTypeMinimumCommitment,
		TermTypeUsageCap,
		TermTypeDiscount,
		TermTypeRebate,
		TermTypePenalty,
		TermTypeSLACredit,
		TermTypePaymentTerms,
		TermTypeRenewalRule,
		TermTypeInvoiceSchedule:
		return nil
	default:
		return ErrTermTypeInvalid
	}
}
