package ingestion

import (
	"context"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
)

// Store defines persistence operations for ingestion.
type Store interface {
	UpsertUsageRecords(ctx context.Context, records []billing.UsageRecord) error
	UpsertInvoice(ctx context.Context, invoice billing.Invoice, lines []billing.InvoiceLine) error
}
