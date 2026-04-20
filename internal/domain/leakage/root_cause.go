package leakage

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// Category classifies the broad operational reason behind a leakage case.
type Category string

const (
	CategoryContractConfigError    Category = "contract_config_error"
	CategoryBillingEngineBug       Category = "billing_engine_bug"
	CategoryManualProcessFailure   Category = "manual_process_failure"
	CategoryCRMBillingSyncGap      Category = "crm_billing_sync_gap"
	CategoryPaymentAllocationIssue Category = "payment_allocation_issue"
	CategoryUnauthorizedDiscount   Category = "unauthorized_discount"
	CategoryMissingUsageIngestion  Category = "missing_usage_ingestion"
	CategorySLACompensationGap     Category = "sla_compensation_logic_gap"
)

// DerivedBy identifies which mechanism produced a root-cause hypothesis.
type DerivedBy string

const (
	DerivedByRules DerivedBy = "rules"
	DerivedByAI    DerivedBy = "ai"
	DerivedByHuman DerivedBy = "human"
)

// RootCause captures the most likely explanation for why a leakage case
// occurred.
type RootCause struct {
	ID              uuid.UUID
	CaseID          uuid.UUID
	Category        Category
	Subcategory     string
	Description     string
	ConfidenceScore valueobject.ConfidenceScore
	DerivedBy       DerivedBy
	CreatedAt       time.Time
}
