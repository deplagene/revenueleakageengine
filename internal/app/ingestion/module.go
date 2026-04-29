// Package ingestion wires ingestion application entrypoints and adapters.
package ingestion

import (
	"context"
	"errors"
	"fmt"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

var (
	// ErrStoreRequired reports that ingestion commands were created without a
	// persistence boundary.
	ErrStoreRequired = errors.New("ingestion store is required")
	// ErrUsageRecordsRequired reports that an empty usage batch was submitted.
	ErrUsageRecordsRequired = errors.New("usage records are required")
	// ErrInvoiceRequired reports that invoice ingestion was requested without a
	// usable invoice payload.
	ErrInvoiceRequired = errors.New("invoice is required")
	// ErrInvoiceLinesRequired reports that invoice ingestion was requested
	// without invoice lines.
	ErrInvoiceLinesRequired = errors.New("invoice lines are required")
)

// Commands orchestrates data ingestion.
type Commands struct {
	store Store
}

// NewCommands creates a new commands handler.
func NewCommands(store Store) (*Commands, error) {
	if store == nil {
		return nil, ErrStoreRequired
	}

	return &Commands{
		store: store,
	}, nil
}

// IngestUsageRecords handles a batch of usage records.
func (c *Commands) IngestUsageRecords(ctx context.Context, records []billing.UsageRecord) error {
	const op = "app.ingestion.IngestUsageRecords"

	if err := ctx.Err(); err != nil {
		return err
	}

	if len(records) == 0 {
		return fmt.Errorf("%s: %w", op, ErrUsageRecordsRequired)
	}

	normalized := make([]billing.UsageRecord, 0, len(records))
	for index, record := range records {
		if record.ID == uuid.Nil {
			record.ID = uuid.New()
		}

		record = record.Normalize()
		if err := record.Validate(); err != nil {
			return fmt.Errorf("%s: record[%d]: %w", op, index, err)
		}

		normalized = append(normalized, record)
	}

	if err := c.store.UpsertUsageRecords(ctx, normalized); err != nil {
		return fmt.Errorf("%s: upsert usage records: %w", op, err)
	}

	return nil
}

// IngestInvoice handles an invoice and its lines.
func (c *Commands) IngestInvoice(ctx context.Context, invoice billing.Invoice, lines []billing.InvoiceLine) error {
	const op = "app.ingestion.IngestInvoice"

	if err := ctx.Err(); err != nil {
		return err
	}

	if invoice == (billing.Invoice{}) {
		return fmt.Errorf("%s: %w", op, ErrInvoiceRequired)
	}

	if len(lines) == 0 {
		return fmt.Errorf("%s: %w", op, ErrInvoiceLinesRequired)
	}

	if invoice.ID == uuid.Nil {
		invoice.ID = uuid.New()
	}

	invoice = invoice.Normalize()
	if err := invoice.Validate(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	normalizedLines := make([]billing.InvoiceLine, 0, len(lines))
	for index, line := range lines {
		if line.ID == uuid.Nil {
			line.ID = uuid.New()
		}

		line = line.Normalize(invoice.ID)
		if err := line.Validate(invoice.TotalAmount.Currency); err != nil {
			return fmt.Errorf("%s: line[%d]: %w", op, index, err)
		}

		normalizedLines = append(normalizedLines, line)
	}

	if err := c.store.UpsertInvoice(ctx, invoice, normalizedLines); err != nil {
		return fmt.Errorf("%s: upsert invoice: %w", op, err)
	}

	return nil
}

// Queries orchestrates data retrieval for ingestion facts.
type Queries struct {
	store Store
}

// ContractPeriodQuery identifies ingested facts for one contract and billing
// period.
type ContractPeriodQuery struct {
	TenantID   uuid.UUID
	ContractID uuid.UUID
	Period     valueobject.BillingPeriod
}

// Validate checks that the query can safely address a contract period.
func (q ContractPeriodQuery) Validate() error {
	if q.TenantID == uuid.Nil {
		return billing.ErrTenantRequired
	}

	if q.ContractID == uuid.Nil {
		return billing.ErrContractRequired
	}

	if err := q.Period.Validate(); err != nil {
		return err
	}

	return nil
}

// NewQueries creates a new queries handler.
func NewQueries(store Store) (*Queries, error) {
	if store == nil {
		return nil, ErrStoreRequired
	}

	return &Queries{
		store: store,
	}, nil
}

func (q *Queries) ListUsageRecordsForContractPeriod(
	ctx context.Context,
	query ContractPeriodQuery,
) ([]billing.UsageRecord, error) {
	const op = "app.ingestion.ListUsageRecordsForContractPeriod"

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return q.store.ListUsageRecordsForContractPeriod(ctx, query)
}

func (q *Queries) ListInvoicesForContractPeriod(
	ctx context.Context,
	query ContractPeriodQuery,
) ([]billing.Invoice, []billing.InvoiceLine, error) {
	const op = "app.ingestion.ListInvoicesForContractPeriod"

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	if err := query.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", op, err)
	}

	return q.store.ListInvoicesForContractPeriod(ctx, query)
}
