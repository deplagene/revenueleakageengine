package revenue

import (
	"errors"
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

func TestInvoiceActualRevenueBuilderBuildActualRevenue(t *testing.T) {
	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	invoiceID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")
	recognizedAt := mustTime(t, "2026-04-30T23:00:00Z")

	builder := NewInvoiceActualRevenueBuilder()
	entries, err := builder.BuildActualRevenue(BuildActualRevenueCommand{
		TenantID:   tenantID,
		CustomerID: customerID,
		ContractID: contractID,
		Period:     period,
		Invoices: []billing.Invoice{
			issuedInvoice(invoiceID, tenantID, customerID, contractID, period),
		},
		InvoiceLines: []billing.InvoiceLine{
			invoiceLine(invoiceID, billableItemID, valueobject.MustMoney("USD", 400_000)),
			invoiceLine(invoiceID, billableItemID, valueobject.MustMoney("USD", 64_000)),
		},
		RecognizedAt: recognizedAt,
		TraceID:      "trace-actual-1",
	})
	if err != nil {
		t.Fatalf("BuildActualRevenue() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}

	entry := entries[0]
	if entry.ActualAmount.MinorUnits != 464_000 {
		t.Fatalf("actual amount = %d, want 464000", entry.ActualAmount.MinorUnits)
	}

	if entry.RecognizedFrom != revenuedomain.RecognizedFromInvoice {
		t.Fatalf("recognized from = %q, want invoice", entry.RecognizedFrom)
	}

	if !entry.RecognizedAt.Equal(recognizedAt) {
		t.Fatalf("recognized at = %s, want %s", entry.RecognizedAt, recognizedAt)
	}
}

func TestInvoiceActualRevenueBuilderRejectsUnmatchedInvoiceLine(t *testing.T) {
	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")

	builder := NewInvoiceActualRevenueBuilder()
	_, err := builder.BuildActualRevenue(BuildActualRevenueCommand{
		TenantID:   tenantID,
		CustomerID: customerID,
		ContractID: contractID,
		Period:     period,
		Invoices:   []billing.Invoice{},
		InvoiceLines: []billing.InvoiceLine{
			invoiceLine(uuid.New(), uuid.New(), valueobject.MustMoney("USD", 100)),
		},
		RecognizedAt: time.Now(),
	})
	if !errors.Is(err, ErrInvoiceLineMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrInvoiceLineMismatch)
	}
}

func TestInvoiceActualRevenueBuilderIgnoresDraftInvoices(t *testing.T) {
	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	invoiceID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")

	invoice := issuedInvoice(invoiceID, tenantID, customerID, contractID, period)
	invoice.Status = billing.InvoiceStatusDraft

	builder := NewInvoiceActualRevenueBuilder()
	entries, err := builder.BuildActualRevenue(BuildActualRevenueCommand{
		TenantID:   tenantID,
		CustomerID: customerID,
		ContractID: contractID,
		Period:     period,
		Invoices:   []billing.Invoice{invoice},
		InvoiceLines: []billing.InvoiceLine{
			invoiceLine(invoiceID, billableItemID, valueobject.MustMoney("USD", 100)),
		},
		RecognizedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("BuildActualRevenue() error = %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("entry count = %d, want 0", len(entries))
	}
}

func issuedInvoice(
	invoiceID uuid.UUID,
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) billing.Invoice {
	return billing.Invoice{
		ID:          invoiceID,
		TenantID:    tenantID,
		CustomerID:  customerID,
		ContractID:  contractID,
		Period:      period,
		TotalAmount: valueobject.MustMoney("USD", 464_000),
		Status:      billing.InvoiceStatusIssued,
	}
}

func invoiceLine(
	invoiceID uuid.UUID,
	billableItemID uuid.UUID,
	lineTotal valueobject.Money,
) billing.InvoiceLine {
	return billing.InvoiceLine{
		ID:             uuid.New(),
		InvoiceID:      invoiceID,
		BillableItemID: billableItemID,
		LineTotal:      lineTotal,
	}
}
