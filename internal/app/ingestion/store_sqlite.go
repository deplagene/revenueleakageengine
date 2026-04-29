package ingestion

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
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
	defer rollbackUncommitted(tx, &committed)

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
	defer rollbackUncommitted(tx, &committed)

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

func (s *SQLiteStore) ListUsageRecordsForContractPeriod(
	ctx context.Context,
	query ContractPeriodQuery,
) ([]billing.UsageRecord, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListUsageRecordsForContractPeriod(ctx, sqlitedb.ListUsageRecordsForContractPeriodParams{
		TenantID:    query.TenantID.String(),
		ContractID:  query.ContractID.String(),
		UsageTime:   formatStoredTime(query.Period.Start),
		UsageTime_2: formatStoredTime(query.Period.End),
	})
	if err != nil {
		return nil, fmt.Errorf("list usage records: %w", err)
	}

	records := make([]billing.UsageRecord, 0, len(rows))
	for index, row := range rows {
		id, err := parseStoredUUID("usage record id", row.ID)
		if err != nil {
			return nil, fmt.Errorf("usage row[%d]: %w", index, err)
		}

		tenantID, err := parseStoredUUID("usage tenant id", row.TenantID)
		if err != nil {
			return nil, fmt.Errorf("usage row[%d]: %w", index, err)
		}

		customerID, err := parseStoredUUID("usage customer id", row.CustomerID)
		if err != nil {
			return nil, fmt.Errorf("usage row[%d]: %w", index, err)
		}

		contractID, err := parseStoredUUID("usage contract id", row.ContractID)
		if err != nil {
			return nil, fmt.Errorf("usage row[%d]: %w", index, err)
		}

		billableItemID, err := parseStoredUUID("usage billable item id", row.BillableItemID)
		if err != nil {
			return nil, fmt.Errorf("usage row[%d]: %w", index, err)
		}

		usageTime, err := parseStoredTime("usage time", row.UsageTime)
		if err != nil {
			return nil, fmt.Errorf("usage row[%d]: %w", index, err)
		}

		metadata, err := decodeJSONMap("usage metadata", row.MetadataJson)
		if err != nil {
			return nil, fmt.Errorf("usage row[%d]: %w", index, err)
		}

		record := billing.UsageRecord{
			ID:             id,
			TenantID:       tenantID,
			CustomerID:     customerID,
			ContractID:     contractID,
			BillableItemID: billableItemID,
			ExternalID:     row.ExternalID,
			UsageTime:      usageTime,
			Quantity:       row.Quantity,
			Unit:           row.Unit,
			SourceSystem:   row.SourceSystem,
			TraceID:        row.TraceID,
			Metadata:       metadata,
		}.Normalize()
		if err := record.Validate(); err != nil {
			return nil, fmt.Errorf("usage row[%d]: %w", index, err)
		}

		records = append(records, record)
	}

	return records, nil
}

func (s *SQLiteStore) ListInvoicesForContractPeriod(
	ctx context.Context,
	query ContractPeriodQuery,
) ([]billing.Invoice, []billing.InvoiceLine, error) {
	if err := query.Validate(); err != nil {
		return nil, nil, err
	}

	rows, err := s.queries.ListInvoicesForContractPeriod(ctx, sqlitedb.ListInvoicesForContractPeriodParams{
		TenantID:    query.TenantID.String(),
		ContractID:  query.ContractID.String(),
		PeriodStart: formatStoredTime(query.Period.Start),
		PeriodEnd:   formatStoredTime(query.Period.End),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list invoices: %w", err)
	}

	invoices := make([]billing.Invoice, 0, len(rows))
	allLines := make([]billing.InvoiceLine, 0)

	for index, row := range rows {
		id, err := parseStoredUUID("invoice id", row.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		tenantID, err := parseStoredUUID("invoice tenant id", row.TenantID)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		customerID, err := parseStoredUUID("invoice customer id", row.CustomerID)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		contractID, err := parseStoredUUID("invoice contract id", row.ContractID)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		periodStart, err := parseStoredTime("invoice period start", row.PeriodStart)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		periodEnd, err := parseStoredTime("invoice period end", row.PeriodEnd)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		issuedAt, err := parseStoredTime("invoice issued_at", row.IssuedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		dueAt, err := parseNullableStoredTime("invoice due_at", row.DueAt)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		period, err := valueobject.NewBillingPeriod(periodStart, periodEnd)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: build period: %w", index, err)
		}

		amount, err := valueobject.NewMoney(row.Currency, row.TotalAmountMinorUnits)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: build total amount: %w", index, err)
		}

		invoice := billing.Invoice{
			ID:           id,
			TenantID:     tenantID,
			CustomerID:   customerID,
			ContractID:   contractID,
			ExternalID:   row.ExternalID,
			Number:       row.InvoiceNumber,
			Period:       period,
			IssuedAt:     issuedAt,
			DueAt:        dueAt,
			TotalAmount:  amount,
			Status:       billing.InvoiceStatus(row.Status),
			SourceSystem: row.SourceSystem,
		}.Normalize()
		if err := invoice.Validate(); err != nil {
			return nil, nil, fmt.Errorf("invoice row[%d]: %w", index, err)
		}

		invoices = append(invoices, invoice)

		lineRows, err := s.queries.ListInvoiceLinesByInvoice(ctx, row.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("list invoice %s lines: %w", id, err)
		}

		for lineIndex, lineRow := range lineRows {
			line, err := invoiceLineFromRow(lineRow, id, invoice.TotalAmount.Currency)
			if err != nil {
				return nil, nil, fmt.Errorf("invoice row[%d] line[%d]: %w", index, lineIndex, err)
			}

			allLines = append(allLines, line)
		}
	}

	return invoices, allLines, nil
}

func invoiceLineFromRow(
	row sqlitedb.InvoiceLine,
	invoiceID uuid.UUID,
	invoiceCurrency string,
) (billing.InvoiceLine, error) {
	lineID, err := parseStoredUUID("invoice line id", row.ID)
	if err != nil {
		return billing.InvoiceLine{}, err
	}

	billableItemID, err := parseNullableStoredUUID("invoice line billable item id", row.BillableItemID)
	if err != nil {
		return billing.InvoiceLine{}, err
	}

	unitPrice, err := valueobject.NewMoney(row.Currency, row.UnitPriceMinorUnits)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build unit price: %w", err)
	}

	discountAmount, err := valueobject.NewMoney(row.Currency, row.DiscountAmountMinorUnits)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build discount amount: %w", err)
	}

	taxAmount, err := valueobject.NewMoney(row.Currency, row.TaxAmountMinorUnits)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build tax amount: %w", err)
	}

	lineTotal, err := valueobject.NewMoney(row.Currency, row.LineTotalMinorUnits)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build line total: %w", err)
	}

	pricingSnapshot, err := decodeJSONMap("invoice line pricing snapshot", row.PricingSnapshotJson)
	if err != nil {
		return billing.InvoiceLine{}, err
	}

	line := billing.InvoiceLine{
		ID:              lineID,
		InvoiceID:       invoiceID,
		BillableItemID:  billableItemID,
		Description:     row.Description,
		Quantity:        row.Quantity,
		UnitPrice:       unitPrice,
		DiscountAmount:  discountAmount,
		TaxAmount:       taxAmount,
		LineTotal:       lineTotal,
		SourceRef:       row.SourceRef,
		PricingSnapshot: pricingSnapshot,
	}.Normalize(invoiceID)
	if err := line.Validate(invoiceCurrency); err != nil {
		return billing.InvoiceLine{}, err
	}

	return line, nil
}

func parseStoredUUID(field, value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse %s: %w", field, err)
	}

	return parsed, nil
}

func rollbackUncommitted(tx *sql.Tx, committed *bool) {
	if *committed {
		return
	}

	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return
	}
}

func parseNullableStoredUUID(field string, value sql.NullString) (uuid.UUID, error) {
	if !value.Valid || value.String == "" {
		return uuid.Nil, nil
	}

	return parseStoredUUID(field, value.String)
}

func parseStoredTime(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", field, err)
	}

	return parsed.UTC(), nil
}

func parseNullableStoredTime(field string, value sql.NullString) (time.Time, error) {
	if !value.Valid || value.String == "" {
		return time.Time{}, nil
	}

	return parseStoredTime(field, value.String)
}

func decodeJSONMap(field, value string) (map[string]any, error) {
	if value == "" {
		return map[string]any{}, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil, fmt.Errorf("decode %s json: %w", field, err)
	}

	if decoded == nil {
		return map[string]any{}, nil
	}

	return decoded, nil
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
