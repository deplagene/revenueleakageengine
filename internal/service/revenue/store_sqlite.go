package revenue

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	sqlitedb "github.com/deplagene/revenueleakageengine/internal/platform/sqlite/sqlc"
)

var _ Store = (*SQLiteStore)(nil)

// SQLiteStore persists revenue ledger entries through SQLite-backed sqlc
// queries.
type SQLiteStore struct {
	queries *sqlitedb.Queries
}

// NewSQLiteStore creates a revenue store backed by a SQLite database.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{
		queries: sqlitedb.New(db),
	}
}

// SaveExpectedRevenue stores one expected revenue ledger entry.
func (s *SQLiteStore) SaveExpectedRevenue(
	ctx context.Context,
	entry revenuedomain.ExpectedRevenueEntry,
) error {
	calculationBasis, err := encodeJSONMap("calculation basis", entry.CalculationBasis)
	if err != nil {
		return err
	}

	if err := s.queries.CreateExpectedRevenueEntry(ctx, sqlitedb.CreateExpectedRevenueEntryParams{
		ID:                       entry.ID.String(),
		TenantID:                 entry.TenantID.String(),
		CustomerID:               entry.CustomerID.String(),
		ContractID:               entry.ContractID.String(),
		BillableItemID:           entry.BillableItemID.String(),
		PeriodStart:              formatStoredTime(entry.Period.Start),
		PeriodEnd:                formatStoredTime(entry.Period.End),
		ExpectedAmountMinorUnits: entry.ExpectedAmount.MinorUnits,
		Currency:                 entry.ExpectedAmount.Currency,
		CalculationBasisJson:     calculationBasis,
		CalculatedAt:             formatStoredTime(entry.CalculatedAt),
		Version:                  int64(entry.Version),
		TraceID:                  entry.TraceID,
	}); err != nil {
		return fmt.Errorf("create expected revenue entry: %w", err)
	}

	return nil
}

// SaveActualRevenue stores one actual revenue ledger entry.
func (s *SQLiteStore) SaveActualRevenue(
	ctx context.Context,
	entry revenuedomain.ActualRevenueEntry,
) error {
	if err := s.queries.CreateActualRevenueEntry(ctx, sqlitedb.CreateActualRevenueEntryParams{
		ID:                     entry.ID.String(),
		TenantID:               entry.TenantID.String(),
		CustomerID:             entry.CustomerID.String(),
		ContractID:             entry.ContractID.String(),
		BillableItemID:         entry.BillableItemID.String(),
		PeriodStart:            formatStoredTime(entry.Period.Start),
		PeriodEnd:              formatStoredTime(entry.Period.End),
		ActualAmountMinorUnits: entry.ActualAmount.MinorUnits,
		Currency:               entry.ActualAmount.Currency,
		RecognizedFrom:         string(entry.RecognizedFrom),
		RecognizedAt:           formatStoredTime(entry.RecognizedAt),
		TraceID:                entry.TraceID,
	}); err != nil {
		return fmt.Errorf("create actual revenue entry: %w", err)
	}

	return nil
}

func formatStoredTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
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
