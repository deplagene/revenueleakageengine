package leakage

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

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
	CaseTypeSLACreditNotAccounted       CaseType = "sla_credit_not_accounted"
	CaseTypePricingRuleMisapplied       CaseType = "pricing_rule_misapplied"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Status string

const (
	StatusOpen          Status = "open"
	StatusInvestigating Status = "investigating"
	StatusResolved      Status = "resolved"
	StatusDismissed     Status = "dismissed"
)

type Case struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	CustomerID        uuid.UUID
	ContractID        uuid.UUID
	Type              CaseType
	Severity          Severity
	Status            Status
	DetectedAt        time.Time
	Period            valueobject.BillingPeriod
	ExpectedAmount    valueobject.Money
	ActualAmount      valueobject.Money
	LeakageAmount     valueobject.Money
	ConfidenceScore   valueobject.ConfidenceScore
	RootCauseCategory Category
	Assignee          string
	TraceID           string
}
