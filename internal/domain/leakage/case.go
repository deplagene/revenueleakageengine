// Package leakage defines investigation artifacts created when expected and
// actual revenue diverge in a meaningful way.
package leakage

import (
	"errors"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// CaseType identifies the mismatch pattern detected during reconciliation.
type CaseType string

const (
	CaseTypeUnderbilling                CaseType = "underbilling"
	CaseTypeUnbilledUsage               CaseType = "unbilled_usage"
	CaseTypeExpiredDiscountStillApplied CaseType = "expired_discount_still_applied"
	CaseTypeContractBillingMismatch     CaseType = "contract_billing_mismatch"
	CaseTypeMissingInvoice              CaseType = "missing_invoice"
	CaseTypePaymentShortfall            CaseType = "payment_shortfall"
	CaseTypeUnclaimedRebate             CaseType = "unclaimed_rebate"
	CaseTypeOverCrediting               CaseType = "over_crediting"
	//nolint:gosec // SLA is a service-level agreement acronym, not a credential.
	CaseTypeSLACreditNotAccounted CaseType = "sla_credit_not_accounted"
	CaseTypePricingRuleMisapplied CaseType = "pricing_rule_misapplied"
)

// Severity expresses the business urgency of a leakage case.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Status represents the investigation lifecycle state of a leakage case.
type Status string

const (
	StatusOpen          Status = "open"
	StatusInvestigating Status = "investigating"
	StatusResolved      Status = "resolved"
	StatusDismissed     Status = "dismissed"
)

var (
	// ErrInvalidStatus reports that a case status falls outside the supported
	// lifecycle values.
	ErrInvalidStatus = errors.New("invalid case status")
	// ErrInvalidStatusTransition reports that a case status change violates the
	// supported lifecycle transitions.
	ErrInvalidStatusTransition = errors.New("invalid case status transition")
)

// Validate checks that the case status is one of the supported lifecycle
// values.
func (s Status) Validate() error {
	switch s {
	case StatusOpen, StatusInvestigating, StatusResolved, StatusDismissed:
		return nil
	default:
		return ErrInvalidStatus
	}
}

// CanTransitionTo reports whether a case may move from the current status to
// the next status.
func (s Status) CanTransitionTo(next Status) bool {
	if s == next {
		return true
	}

	switch s {
	case StatusOpen:
		return next == StatusInvestigating || next == StatusResolved || next == StatusDismissed
	case StatusInvestigating:
		return next == StatusOpen || next == StatusResolved || next == StatusDismissed
	case StatusResolved, StatusDismissed:
		return next == StatusInvestigating
	default:
		return false
	}
}

// Case is the primary investigation record created from a revenue mismatch.
type Case struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	CustomerID          uuid.UUID
	ContractID          uuid.UUID
	ReconciliationRunID uuid.UUID
	Type                CaseType
	Severity            Severity
	Status              Status
	DetectedAt          time.Time
	Period              valueobject.BillingPeriod
	ExpectedAmount      valueobject.Money
	ActualAmount        valueobject.Money
	LeakageAmount       valueobject.Money
	ConfidenceScore     valueobject.ConfidenceScore
	RootCauseCategory   Category
	Assignee            string
	TraceID             string
}
