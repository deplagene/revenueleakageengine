package event

import (
	"time"

	"github.com/google/uuid"
)

type BillingPeriodPayload struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type MoneyPayload struct {
	Currency   string `json:"currency"`
	MinorUnits int64  `json:"minor_units"`
}

type UsageRecordPayload struct {
	ID             uuid.UUID      `json:"id,omitempty"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	CustomerID     uuid.UUID      `json:"customer_id"`
	ContractID     uuid.UUID      `json:"contract_id"`
	BillableItemID uuid.UUID      `json:"billable_item_id"`
	ExternalID     string         `json:"external_id"`
	UsageTime      time.Time      `json:"usage_time"`
	Quantity       int64          `json:"quantity"`
	Unit           string         `json:"unit"`
	SourceSystem   string         `json:"source_system"`
	TraceID        string         `json:"trace_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type InvoicePayload struct {
	ID           uuid.UUID            `json:"id,omitempty"`
	TenantID     uuid.UUID            `json:"tenant_id"`
	CustomerID   uuid.UUID            `json:"customer_id"`
	ContractID   uuid.UUID            `json:"contract_id"`
	ExternalID   string               `json:"external_id"`
	Number       string               `json:"number"`
	Period       BillingPeriodPayload `json:"period"`
	IssuedAt     time.Time            `json:"issued_at"`
	DueAt        time.Time            `json:"due_at,omitempty"`
	TotalAmount  MoneyPayload         `json:"total_amount"`
	Status       string               `json:"status"`
	SourceSystem string               `json:"source_system"`
	Lines        []InvoiceLinePayload `json:"lines"`
	TraceID      string               `json:"trace_id,omitempty"`
}

type InvoiceLinePayload struct {
	ID              uuid.UUID      `json:"id,omitempty"`
	BillableItemID  uuid.UUID      `json:"billable_item_id,omitempty"`
	Description     string         `json:"description"`
	Quantity        int64          `json:"quantity"`
	UnitPrice       MoneyPayload   `json:"unit_price"`
	DiscountAmount  MoneyPayload   `json:"discount_amount"`
	TaxAmount       MoneyPayload   `json:"tax_amount"`
	LineTotal       MoneyPayload   `json:"line_total"`
	SourceRef       string         `json:"source_ref"`
	PricingSnapshot map[string]any `json:"pricing_snapshot,omitempty"`
}

type ReconciliationRunRequestedPayload struct {
	RunID                    uuid.UUID            `json:"run_id,omitempty"`
	TenantID                 uuid.UUID            `json:"tenant_id"`
	ContractID               uuid.UUID            `json:"contract_id"`
	Period                   BillingPeriodPayload `json:"period"`
	Currency                 string               `json:"currency"`
	MinimumLeakageMinorUnits int64                `json:"minimum_leakage_minor_units"`
	TraceID                  string               `json:"trace_id,omitempty"`
}

type ReconciliationRunCompletedPayload struct {
	RunID            uuid.UUID    `json:"run_id"`
	TenantID         uuid.UUID    `json:"tenant_id"`
	ContractID       uuid.UUID    `json:"contract_id"`
	ExpectedEntryID  uuid.UUID    `json:"expected_entry_id"`
	ActualEntryCount int          `json:"actual_entry_count"`
	DiffCount        int          `json:"diff_count"`
	CaseCount        int          `json:"case_count"`
	LeakageAmount    MoneyPayload `json:"leakage_amount"`
	TraceID          string       `json:"trace_id,omitempty"`
}

type LeakageCaseCreatedPayload struct {
	CaseID              uuid.UUID    `json:"case_id"`
	TenantID            uuid.UUID    `json:"tenant_id"`
	CustomerID          uuid.UUID    `json:"customer_id"`
	ContractID          uuid.UUID    `json:"contract_id"`
	ReconciliationRunID uuid.UUID    `json:"reconciliation_run_id"`
	Type                string       `json:"type"`
	Severity            string       `json:"severity"`
	Status              string       `json:"status"`
	LeakageAmount       MoneyPayload `json:"leakage_amount"`
	TraceID             string       `json:"trace_id,omitempty"`
}
