package ingestion

import (
	"context"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
)

// Store defines persistence operations for ingestion.
type Store interface {
	UpsertUsageRecord(ctx context.Context, u *billing.UsageRecord) error
	UpsertInvoice(ctx context.Context, inv *billing.Invoice, lines []billing.InvoiceLine) error
}
