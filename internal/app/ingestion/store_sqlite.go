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

// UpsertUsageRecords inserts or updates usage records by external id and source
// system in one transaction.
func (s *SQLiteStore) UpsertUsageRecords(ctx context.Context, records []billing.UsageRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin usage records upsert tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	q := s.queries.WithTx(tx)
	now := formatStoredTime(time.Now())

	for _, record := range records {
		metadata, err := encodeJSONMap("usage metadata", record.Metadata)
		if err != nil {
			return err
		}

		err = q.UpsertUsageRecord(ctx, sqlitedb.UpsertUsageRecordParams{
			ID:             record.ID.String(),
			TenantID:       record.TenantID.String(),
			CustomerID:     record.CustomerID.String(),
			ContractID:     record.ContractID.String(),
			BillableItemID: record.BillableItemID.String(),
			ExternalID:     record.ExternalID,
			UsageTime:      formatStoredTime(record.UsageTime),
			Quantity:       record.Quantity,
			Unit:           record.Unit,
			SourceSystem:   record.SourceSystem,
			TraceID:        record.TraceID,
			MetadataJson:   metadata,
			CreatedAt:      now,
		})
		if err != nil {
			return fmt.Errorf("upsert usage record %q: %w", record.ExternalID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit usage records upsert tx: %w", err)
	}

	committed = true
	return nil
}

// UpsertInvoice inserts or updates an invoice and its lines by external id and source system.
func (s *SQLiteStore) UpsertInvoice(ctx context.Context, inv billing.Invoice, lines []billing.InvoiceLine) error {
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
	now := formatStoredTime(time.Now())

	err = q.UpsertInvoice(ctx, sqlitedb.UpsertInvoiceParams{
		ID:                    inv.ID.String(),
		TenantID:              inv.TenantID.String(),
		CustomerID:            inv.CustomerID.String(),
		ContractID:            inv.ContractID.String(),
		ExternalID:            inv.ExternalID,
		InvoiceNumber:         inv.Number,
		PeriodStart:           formatStoredTime(inv.Period.Start),
		PeriodEnd:             formatStoredTime(inv.Period.End),
		IssuedAt:              formatStoredTime(inv.IssuedAt),
		DueAt:                 nullableStoredTime(inv.DueAt),
		Currency:              inv.TotalAmount.Currency,
		TotalAmountMinorUnits: inv.TotalAmount.MinorUnits,
		Status:                string(inv.Status),
		SourceSystem:          inv.SourceSystem,
		CreatedAt:             now,
	})
	if err != nil {
		return fmt.Errorf("upsert invoice: %w", err)
	}

	persistedInvoice, err := q.GetInvoiceBySourceExternal(ctx, sqlitedb.GetInvoiceBySourceExternalParams{
		TenantID:     inv.TenantID.String(),
		SourceSystem: inv.SourceSystem,
		ExternalID:   inv.ExternalID,
	})
	if err != nil {
		return fmt.Errorf("get persisted invoice: %w", err)
	}

	err = q.DeleteInvoiceLinesByInvoice(ctx, persistedInvoice.ID)
	if err != nil {
		return fmt.Errorf("delete old invoice lines: %w", err)
	}

	for _, line := range lines {
		pricing, err := encodeJSONMap("invoice line pricing snapshot", line.PricingSnapshot)
		if err != nil {
			return err
		}

		err = q.CreateInvoiceLine(ctx, sqlitedb.CreateInvoiceLineParams{
			ID:                       line.ID.String(),
			InvoiceID:                persistedInvoice.ID,
			TenantID:                 inv.TenantID.String(),
			BillableItemID:           nullableUUID(line.BillableItemID),
			Description:              line.Description,
			Quantity:                 line.Quantity,
			UnitPriceMinorUnits:      line.UnitPrice.MinorUnits,
			DiscountAmountMinorUnits: line.DiscountAmount.MinorUnits,
			TaxAmountMinorUnits:      line.TaxAmount.MinorUnits,
			LineTotalMinorUnits:      line.LineTotal.MinorUnits,
			Currency:                 line.UnitPrice.Currency,
			SourceRef:                line.SourceRef,
			PricingSnapshotJson:      pricing,
			CreatedAt:                now,
		})
		if err != nil {
			return fmt.Errorf("create invoice line %q: %w", line.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit invoice upsert tx: %w", err)
	}
	committed = true

	return nil
}

func encodeJSONMap(field string, value map[string]any) (string, error) {
	if value == nil {
		return "{}", nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode %s json: %w", field, err)
	}

	return string(encoded), nil
}

func formatStoredTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableStoredTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}

	return sql.NullString{
		String: formatStoredTime(value),
		Valid:  true,
	}
}

func nullableUUID(value uuid.UUID) sql.NullString {
	if value == uuid.Nil {
		return sql.NullString{}
	}

	return sql.NullString{
		String: value.String(),
		Valid:  true,
	}
}
