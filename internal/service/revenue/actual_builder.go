package revenue

import (
	"errors"
	"fmt"
	"sort"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

var (
	// ErrInvoiceMismatch reports an invoice that does not belong to the command
	// tenant, customer, contract, or billing period.
	ErrInvoiceMismatch = errors.New("invoice does not match actual revenue context")
	// ErrInvoiceLineMismatch reports an invoice line that cannot be linked to a
	// selected invoice.
	ErrInvoiceLineMismatch = errors.New("invoice line does not match selected invoices")
	// ErrInvoiceLineBillableItemRequired reports an invoice line without a
	// billable item.
	ErrInvoiceLineBillableItemRequired = errors.New("invoice line billable item id is required")
	// ErrInvoiceLineCurrencyRequired reports an invoice line without money
	// currency.
	ErrInvoiceLineCurrencyRequired = errors.New("invoice line currency is required")
)

// ActualRevenueBuilder constructs actual revenue ledger entries from normalized
// billing facts.
type ActualRevenueBuilder interface {
	BuildActualRevenue(cmd BuildActualRevenueCommand) ([]revenuedomain.ActualRevenueEntry, error)
}

type invoiceActualRevenueBuilder struct{}

// NewInvoiceActualRevenueBuilder creates the MVP actual revenue builder based
// on invoice lines.
func NewInvoiceActualRevenueBuilder() ActualRevenueBuilder {
	return invoiceActualRevenueBuilder{}
}

// BuildActualRevenue aggregates issued invoice lines into actual revenue
// entries by billable item.
func (b invoiceActualRevenueBuilder) BuildActualRevenue(
	cmd BuildActualRevenueCommand,
) ([]revenuedomain.ActualRevenueEntry, error) {
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	selectedInvoices := make(map[uuid.UUID]billing.Invoice, len(cmd.Invoices))
	ignoredInvoices := map[uuid.UUID]struct{}{}
	for _, invoice := range cmd.Invoices {
		if invoice.Status == billing.InvoiceStatusDraft || invoice.Status == billing.InvoiceStatusVoid {
			ignoredInvoices[invoice.ID] = struct{}{}
			continue
		}

		if !invoiceMatchesCommand(invoice, cmd) {
			return nil, fmt.Errorf("%w: invoice_id=%s", ErrInvoiceMismatch, invoice.ID)
		}

		selectedInvoices[invoice.ID] = invoice
	}

	amountsByBillableItem := map[uuid.UUID]valueobject.Money{}
	for _, line := range cmd.InvoiceLines {
		if _, ok := ignoredInvoices[line.InvoiceID]; ok {
			continue
		}

		if _, ok := selectedInvoices[line.InvoiceID]; !ok {
			return nil, fmt.Errorf("%w: invoice_line_id=%s", ErrInvoiceLineMismatch, line.ID)
		}

		if line.BillableItemID == uuid.Nil {
			return nil, fmt.Errorf("%w: invoice_line_id=%s", ErrInvoiceLineBillableItemRequired, line.ID)
		}

		if line.LineTotal.Currency == "" {
			return nil, fmt.Errorf("%w: invoice_line_id=%s", ErrInvoiceLineCurrencyRequired, line.ID)
		}

		current, ok := amountsByBillableItem[line.BillableItemID]
		if !ok {
			amountsByBillableItem[line.BillableItemID] = line.LineTotal
			continue
		}

		next, err := current.Add(line.LineTotal)
		if err != nil {
			return nil, fmt.Errorf("sum invoice line totals: %w", err)
		}

		amountsByBillableItem[line.BillableItemID] = next
	}

	entries := make([]revenuedomain.ActualRevenueEntry, 0, len(amountsByBillableItem))
	for _, billableItemID := range sortedBillableItemIDs(amountsByBillableItem) {
		entries = append(entries, revenuedomain.ActualRevenueEntry{
			ID:             uuid.New(),
			TenantID:       cmd.TenantID,
			CustomerID:     cmd.CustomerID,
			ContractID:     cmd.ContractID,
			BillableItemID: billableItemID,
			Period:         cmd.Period,
			ActualAmount:   amountsByBillableItem[billableItemID],
			RecognizedFrom: revenuedomain.RecognizedFromInvoice,
			RecognizedAt:   cmd.RecognizedAt.UTC(),
			TraceID:        cmd.TraceID,
		})
	}

	return entries, nil
}

func invoiceMatchesCommand(invoice billing.Invoice, cmd BuildActualRevenueCommand) bool {
	return invoice.TenantID == cmd.TenantID &&
		invoice.CustomerID == cmd.CustomerID &&
		invoice.ContractID == cmd.ContractID &&
		invoice.Period.Start.Equal(cmd.Period.Start) &&
		invoice.Period.End.Equal(cmd.Period.End)
}

func sortedBillableItemIDs(amounts map[uuid.UUID]valueobject.Money) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(amounts))
	for id := range amounts {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i].String() < ids[j].String()
	})

	return ids
}
