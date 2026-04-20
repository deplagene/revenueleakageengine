// Package billing defines normalized billing-side facts such as invoices, usage,
// adjustments, and SLA events that describe what financially happened.
package billing

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// InvoiceStatus captures the lifecycle state of an invoice.
type InvoiceStatus string

const (
	InvoiceStatusDraft   InvoiceStatus = "draft"
	InvoiceStatusIssued  InvoiceStatus = "issued"
	InvoiceStatusPaid    InvoiceStatus = "paid"
	InvoiceStatusVoid    InvoiceStatus = "void"
	InvoiceStatusOverdue InvoiceStatus = "overdue"
)

// Invoice represents an issued billing document for a contract and billing
// period.
type Invoice struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	CustomerID  uuid.UUID
	ContractID  uuid.UUID
	ExternalID  string
	Number      string
	Period      valueobject.BillingPeriod
	IssuedAt    time.Time
	DueAt       time.Time
	TotalAmount valueobject.Money
	Status      InvoiceStatus
}

// InvoiceLine represents one billable row inside an invoice together with the
// pricing snapshot used to produce it.
type InvoiceLine struct {
	ID              uuid.UUID
	InvoiceID       uuid.UUID
	BillableItemID  uuid.UUID
	Description     string
	Quantity        int64
	UnitPrice       valueobject.Money
	DiscountAmount  valueobject.Money
	TaxAmount       valueobject.Money
	LineTotal       valueobject.Money
	SourceRef       string
	PricingSnapshot map[string]any
}
