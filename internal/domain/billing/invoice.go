// Package billing defines normalized billing-side facts such as invoices, usage,
// adjustments, and SLA events that describe what financially happened.
package billing

import (
	"errors"
	"strings"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

var (
	// ErrInvoiceIDRequired reports that a normalized invoice is missing its
	// internal id.
	ErrInvoiceIDRequired = errors.New("invoice id is required")
	// ErrInvoiceNumberRequired reports that an invoice is missing its business
	// number.
	ErrInvoiceNumberRequired = errors.New("invoice number is required")
	// ErrInvoiceIssuedAtRequired reports that an invoice is missing its issue
	// timestamp.
	ErrInvoiceIssuedAtRequired = errors.New("invoice issued_at is required")
	// ErrInvoiceDueAtInvalid reports a due date that precedes the issue date.
	ErrInvoiceDueAtInvalid = errors.New("invoice due_at cannot be before issued_at")
	// ErrInvoiceStatusRequired reports that an invoice is missing lifecycle
	// status.
	ErrInvoiceStatusRequired = errors.New("invoice status is required")
	// ErrInvoiceStatusInvalid reports an unknown invoice lifecycle status.
	ErrInvoiceStatusInvalid = errors.New("invoice status is invalid")
	// ErrInvoiceLineIDRequired reports that a normalized invoice line is missing
	// its internal id.
	ErrInvoiceLineIDRequired = errors.New("invoice line id is required")
	// ErrInvoiceLineInvoiceIDRequired reports that an invoice line is not linked
	// to an invoice.
	ErrInvoiceLineInvoiceIDRequired = errors.New("invoice line invoice id is required")
	// ErrInvoiceLineDescriptionRequired reports that an invoice line lacks a
	// readable description.
	ErrInvoiceLineDescriptionRequired = errors.New("invoice line description is required")
	// ErrInvoiceLineQuantityInvalid reports a negative invoice line quantity.
	ErrInvoiceLineQuantityInvalid = errors.New("invoice line quantity cannot be negative")
	// ErrInvoiceLineCurrencyMismatch reports line money fields that do not share
	// the invoice currency.
	ErrInvoiceLineCurrencyMismatch = errors.New("invoice line currency mismatch")
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
	ID           uuid.UUID
	TenantID     uuid.UUID
	CustomerID   uuid.UUID
	ContractID   uuid.UUID
	ExternalID   string
	Number       string
	Period       valueobject.BillingPeriod
	IssuedAt     time.Time
	DueAt        time.Time
	TotalAmount  valueobject.Money
	Status       InvoiceStatus
	SourceSystem string
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

// Normalize returns a copy with stable strings and UTC timestamps.
func (i Invoice) Normalize() Invoice {
	i.ExternalID = strings.TrimSpace(i.ExternalID)
	i.Number = strings.TrimSpace(i.Number)
	i.SourceSystem = strings.TrimSpace(i.SourceSystem)

	if !i.IssuedAt.IsZero() {
		i.IssuedAt = i.IssuedAt.UTC()
	}

	if !i.DueAt.IsZero() {
		i.DueAt = i.DueAt.UTC()
	}

	return i
}

// Validate checks that the invoice can be persisted as an idempotent billing
// fact.
func (i Invoice) Validate() error {
	if i.ID == uuid.Nil {
		return ErrInvoiceIDRequired
	}

	if i.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if i.CustomerID == uuid.Nil {
		return ErrCustomerRequired
	}

	if i.ContractID == uuid.Nil {
		return ErrContractRequired
	}

	if strings.TrimSpace(i.ExternalID) == "" {
		return ErrExternalIDRequired
	}

	if strings.TrimSpace(i.SourceSystem) == "" {
		return ErrSourceSystemRequired
	}

	if strings.TrimSpace(i.Number) == "" {
		return ErrInvoiceNumberRequired
	}

	if err := i.Period.Validate(); err != nil {
		return err
	}

	if i.IssuedAt.IsZero() {
		return ErrInvoiceIssuedAtRequired
	}

	if !i.DueAt.IsZero() && i.DueAt.Before(i.IssuedAt) {
		return ErrInvoiceDueAtInvalid
	}

	if err := i.TotalAmount.Validate(); err != nil {
		return err
	}

	if err := i.Status.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate checks that the invoice status is supported.
func (s InvoiceStatus) Validate() error {
	switch s {
	case "":
		return ErrInvoiceStatusRequired
	case InvoiceStatusDraft,
		InvoiceStatusIssued,
		InvoiceStatusPaid,
		InvoiceStatusVoid,
		InvoiceStatusOverdue:
		return nil
	default:
		return ErrInvoiceStatusInvalid
	}
}

// Normalize returns a copy with stable strings and non-nil pricing snapshot.
func (l InvoiceLine) Normalize(invoiceID uuid.UUID) InvoiceLine {
	if l.InvoiceID == uuid.Nil {
		l.InvoiceID = invoiceID
	}

	l.Description = strings.TrimSpace(l.Description)
	l.SourceRef = strings.TrimSpace(l.SourceRef)

	if l.PricingSnapshot == nil {
		l.PricingSnapshot = map[string]any{}
	}

	return l
}

// Validate checks that an invoice line is structurally valid and currency-safe.
func (l InvoiceLine) Validate(invoiceCurrency string) error {
	if l.ID == uuid.Nil {
		return ErrInvoiceLineIDRequired
	}

	if l.InvoiceID == uuid.Nil {
		return ErrInvoiceLineInvoiceIDRequired
	}

	if strings.TrimSpace(l.Description) == "" {
		return ErrInvoiceLineDescriptionRequired
	}

	if l.Quantity < 0 {
		return ErrInvoiceLineQuantityInvalid
	}

	if err := l.UnitPrice.Validate(); err != nil {
		return err
	}

	if err := l.DiscountAmount.Validate(); err != nil {
		return err
	}

	if err := l.TaxAmount.Validate(); err != nil {
		return err
	}

	if err := l.LineTotal.Validate(); err != nil {
		return err
	}

	if l.UnitPrice.Currency != invoiceCurrency ||
		l.DiscountAmount.Currency != invoiceCurrency ||
		l.TaxAmount.Currency != invoiceCurrency ||
		l.LineTotal.Currency != invoiceCurrency {
		return ErrInvoiceLineCurrencyMismatch
	}

	return nil
}
