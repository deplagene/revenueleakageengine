package leakage

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

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

type DerivedBy string

const (
	DerivedByRules DerivedBy = "rules"
	DerivedByAI    DerivedBy = "ai"
	DerivedByHuman DerivedBy = "human"
)

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
