package ingestion

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	sqlitedb "github.com/deplagene/revenueleakageengine/internal/platform/sqlite/sqlc"
	"github.com/google/uuid"
)

var _ Store = (*SQLiteStore)(nil)

// SQLiteStore loads and persists ingested facts using SQLite.
type SQLiteStore struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// NewSQLiteStore creates an ingestion store backed by a SQLite database.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{
		db:      db,
		queries: sqlitedb.New(db),
	}
}

// UpsertUsageRecord inserts or updates a usage record by external id and source system.
func (s *SQLiteStore) UpsertUsageRecord(ctx context.Context, u *billing.UsageRecord) error {
	metadata, err := json.Marshal(u.Metadata)
	if err != nil {
		return fmt.Errorf("marshal usage metadata: %w", err)
	}

	err = s.queries.UpsertUsageRecord(ctx, sqlitedb.UpsertUsageRecordParams{
		ID:             u.ID.String(),
		TenantID:       u.TenantID.String(),
		CustomerID:     u.CustomerID.String(),
		ContractID:     u.ContractID.String(),
		BillableItemID: u.BillableItemID.String(),
		ExternalID:     u.ExternalID,
		UsageTime:      u.UsageTime.UTC().Format(time.RFC3339Nano),
		Quantity:       u.Quantity,
		Unit:           u.Unit,
		SourceSystem:   u.SourceSystem,
		TraceID:        u.TraceID,
		MetadataJson:   string(metadata),
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("upsert usage record: %w", err)
	}

	return nil
}

// UpsertInvoice inserts or updates an invoice and its lines by external id and source system.
func (s *SQLiteStore) UpsertInvoice(ctx context.Context, inv *billing.Invoice, lines []billing.InvoiceLine) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin invoice upsert tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	q := s.queries.WithTx(tx)

	var dueAt sql.NullString
	if !inv.DueAt.IsZero() {
		dueAt = sql.NullString{String: inv.DueAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	err = q.UpsertInvoice(ctx, sqlitedb.UpsertInvoiceParams{
		ID:                    inv.ID.String(),
		TenantID:              inv.TenantID.String(),
		CustomerID:            inv.CustomerID.String(),
		ContractID:            inv.ContractID.String(),
		ExternalID:            inv.ExternalID,
		InvoiceNumber:         inv.Number,
		PeriodStart:           inv.Period.Start.UTC().Format(time.RFC3339Nano),
		PeriodEnd:             inv.Period.End.UTC().Format(time.RFC3339Nano),
		IssuedAt:              inv.IssuedAt.UTC().Format(time.RFC3339Nano),
		DueAt:                 dueAt,
		Currency:              inv.TotalAmount.Currency,
		TotalAmountMinorUnits: inv.TotalAmount.MinorUnits,
		Status:                string(inv.Status),
		SourceSystem:          inv.SourceSystem,
		CreatedAt:             time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("upsert invoice: %w", err)
	}

	err = q.DeleteInvoiceLinesByInvoice(ctx, inv.ID.String())
	if err != nil {
		return fmt.Errorf("delete old invoice lines: %w", err)
	}

	for _, l := range lines {
		pricing, err := json.Marshal(l.PricingSnapshot)
		if err != nil {
			return fmt.Errorf("marshal invoice line pricing snapshot: %w", err)
		}

		var itemID sql.NullString
		if l.BillableItemID != uuid.Nil {
			itemID = sql.NullString{String: l.BillableItemID.String(), Valid: true}
		}

		err = q.CreateInvoiceLine(ctx, sqlitedb.CreateInvoiceLineParams{
			ID:                       l.ID.String(),
			InvoiceID:                inv.ID.String(),
			TenantID:                 inv.TenantID.String(),
			BillableItemID:           itemID,
			Description:              l.Description,
			Quantity:                 l.Quantity,
			UnitPriceMinorUnits:      l.UnitPrice.MinorUnits,
			DiscountAmountMinorUnits: l.DiscountAmount.MinorUnits,
			TaxAmountMinorUnits:      l.TaxAmount.MinorUnits,
			LineTotalMinorUnits:      l.LineTotal.MinorUnits,
			Currency:                 l.UnitPrice.Currency,
			SourceRef:                l.SourceRef,
			PricingSnapshotJson:      string(pricing),
			CreatedAt:                time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return fmt.Errorf("create invoice line: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit invoice upsert tx: %w", err)
	}
	committed = true

	return nil
}
