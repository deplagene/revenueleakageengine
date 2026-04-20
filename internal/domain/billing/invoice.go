package billing

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

type InvoiceStatus string

const (
	InvoiceStatusDraft   InvoiceStatus = "draft"
	InvoiceStatusIssued  InvoiceStatus = "issued"
	InvoiceStatusPaid    InvoiceStatus = "paid"
	InvoiceStatusVoid    InvoiceStatus = "void"
	InvoiceStatusOverdue InvoiceStatus = "overdue"
)

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
