// Package ingestion wires ingestion application entrypoints and adapters.
package ingestion

import (
	"context"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
)

// Commands orchestrates data ingestion.
type Commands struct {
	store Store
}

// NewCommands creates a new commands handler.
func NewCommands(store Store) *Commands {
	return &Commands{
		store: store,
	}
}

// IngestUsageRecords handles a batch of usage records.
func (c *Commands) IngestUsageRecords(ctx context.Context, records []*billing.UsageRecord) error {
	for _, rec := range records {
		if err := c.store.UpsertUsageRecord(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

// IngestInvoice handles an invoice and its lines.
func (c *Commands) IngestInvoice(ctx context.Context, inv *billing.Invoice, lines []billing.InvoiceLine) error {
	return c.store.UpsertInvoice(ctx, inv, lines)
}
