package reconciliation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	sqlitedb "github.com/deplagene/revenueleakageengine/internal/platform/sqlite/sqlc"
	"github.com/google/uuid"
)

var _ Store = (*SQLiteStore)(nil)

// SQLiteStore persists reconciliation data through SQLite-backed sqlc queries.
type SQLiteStore struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// NewSQLiteStore creates a reconciliation store backed by a SQLite database.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{
		db:      db,
		queries: sqlitedb.New(db),
	}
}

// ListReconciliationRuns loads recent reconciliation run summaries.
func (s *SQLiteStore) ListReconciliationRuns(
	ctx context.Context,
	cmd ListReconciliationRunsCommand,
) ([]ReconciliationRunSummary, error) {
	rows, err := s.queries.ListReconciliationRuns(ctx, sqlitedb.ListReconciliationRunsParams{
		TenantID:    nullableUUIDInterface(cmd.TenantID),
		ContractID:  nullableUUIDInterface(cmd.ContractID),
		LimitCount:  int64(cmd.Limit),
		OffsetCount: int64(cmd.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list reconciliation runs: %w", err)
	}

	runs := make([]ReconciliationRunSummary, 0, len(rows))
	for _, row := range rows {
		run, err := reconciliationRunSummaryFromRow(row)
		if err != nil {
			return nil, err
		}

		runs = append(runs, run)
	}

	return runs, nil
}

// GetReconciliationRun loads one reconciliation run summary in tenant scope.
func (s *SQLiteStore) GetReconciliationRun(
	ctx context.Context,
	cmd GetReconciliationRunCommand,
) (ReconciliationRunSummary, error) {
	row, err := s.queries.GetReconciliationRun(ctx, sqlitedb.GetReconciliationRunParams{
		ID:       cmd.RunID.String(),
		TenantID: cmd.TenantID.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReconciliationRunSummary{}, ErrRunNotFound
		}

		return ReconciliationRunSummary{}, fmt.Errorf("get reconciliation run: %w", err)
	}

	return reconciliationRunSummaryFromRow(row)
}

// CreateReconciliationRun stores the initial run record before matching starts.
func (s *SQLiteStore) CreateReconciliationRun(ctx context.Context, run ReconciliationRun) error {
	currency := run.Currency
	if currency == "" {
		currency = run.LeakageAmount.Currency
	}

	if err := s.queries.CreateReconciliationRun(ctx, sqlitedb.CreateReconciliationRunParams{
		ID:                      run.ID.String(),
		TenantID:                run.TenantID.String(),
		ContractID:              run.ContractID.String(),
		PeriodStart:             formatStoredTime(run.Period.Start),
		PeriodEnd:               formatStoredTime(run.Period.End),
		Status:                  string(run.Status),
		StartedAt:               formatStoredTime(run.StartedAt),
		ExpectedCount:           int64(run.ExpectedCount),
		ActualCount:             int64(run.ActualCount),
		DiffCount:               int64(run.DiffCount),
		CaseCount:               int64(run.CaseCount),
		LeakageAmountMinorUnits: run.LeakageAmount.MinorUnits,
		Currency:                currency,
		TraceID:                 run.TraceID,
	}); err != nil {
		return fmt.Errorf("create reconciliation run: %w", err)
	}

	return nil
}

// CompleteReconciliationRun updates the run with final matching statistics.
func (s *SQLiteStore) CompleteReconciliationRun(ctx context.Context, run ReconciliationRun) error {
	if err := s.queries.CompleteReconciliationRun(ctx, sqlitedb.CompleteReconciliationRunParams{
		ID:                      run.ID.String(),
		Status:                  string(run.Status),
		CompletedAt:             nullableStoredTime(run.CompletedAt),
		ExpectedCount:           int64(run.ExpectedCount),
		ActualCount:             int64(run.ActualCount),
		DiffCount:               int64(run.DiffCount),
		CaseCount:               int64(run.CaseCount),
		LeakageAmountMinorUnits: run.LeakageAmount.MinorUnits,
	}); err != nil {
		return fmt.Errorf("complete reconciliation run: %w", err)
	}

	return nil
}

// ListExpectedRevenue loads expected revenue entries for one contract and
// billing period.
func (s *SQLiteStore) ListExpectedRevenue(
	ctx context.Context,
	tenantID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) ([]revenue.ExpectedRevenueEntry, error) {
	rows, err := s.queries.ListExpectedRevenueByContractPeriod(ctx, sqlitedb.ListExpectedRevenueByContractPeriodParams{
		TenantID:    tenantID.String(),
		ContractID:  contractID.String(),
		PeriodStart: formatStoredTime(period.Start),
		PeriodEnd:   formatStoredTime(period.End),
	})
	if err != nil {
		return nil, fmt.Errorf("list expected revenue entries: %w", err)
	}

	entries := make([]revenue.ExpectedRevenueEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := expectedRevenueEntryFromRow(row)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// ListActualRevenue loads actual revenue entries for one contract and billing
// period.
func (s *SQLiteStore) ListActualRevenue(
	ctx context.Context,
	tenantID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) ([]revenue.ActualRevenueEntry, error) {
	rows, err := s.queries.ListActualRevenueByContractPeriod(ctx, sqlitedb.ListActualRevenueByContractPeriodParams{
		TenantID:    tenantID.String(),
		ContractID:  contractID.String(),
		PeriodStart: formatStoredTime(period.Start),
		PeriodEnd:   formatStoredTime(period.End),
	})
	if err != nil {
		return nil, fmt.Errorf("list actual revenue entries: %w", err)
	}

	entries := make([]revenue.ActualRevenueEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := actualRevenueEntryFromRow(row)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// CreateLeakageCase stores a leakage case and its evidence in one transaction.
func (s *SQLiteStore) CreateLeakageCase(ctx context.Context, c leakage.Case, evidence []leakage.Evidence) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin leakage case transaction: %w", err)
	}

	committed := false
	defer rollbackUncommitted(tx, &committed)

	queries := s.queries.WithTx(tx)
	if err := queries.CreateLeakageCase(ctx, sqlitedb.CreateLeakageCaseParams{
		ID:                         c.ID.String(),
		TenantID:                   c.TenantID.String(),
		CustomerID:                 c.CustomerID.String(),
		ContractID:                 c.ContractID.String(),
		ReconciliationRunID:        nullableUUID(c.ReconciliationRunID),
		CaseType:                   string(c.Type),
		Severity:                   string(c.Severity),
		Status:                     string(c.Status),
		DetectedAt:                 formatStoredTime(c.DetectedAt),
		PeriodStart:                formatStoredTime(c.Period.Start),
		PeriodEnd:                  formatStoredTime(c.Period.End),
		ExpectedAmountMinorUnits:   c.ExpectedAmount.MinorUnits,
		ActualAmountMinorUnits:     c.ActualAmount.MinorUnits,
		LeakageAmountMinorUnits:    c.LeakageAmount.MinorUnits,
		Currency:                   c.LeakageAmount.Currency,
		ConfidenceScoreBasisPoints: int64(c.ConfidenceScore.BasisPoints),
		RootCauseCategory:          string(c.RootCauseCategory),
		Assignee:                   c.Assignee,
		TraceID:                    c.TraceID,
	}); err != nil {
		return fmt.Errorf("create leakage case: %w", err)
	}

	for _, item := range evidence {
		payload, err := encodeJSONMap("evidence payload", item.Payload)
		if err != nil {
			return err
		}

		if err := queries.CreateLeakageEvidence(ctx, sqlitedb.CreateLeakageEvidenceParams{
			ID:           item.ID.String(),
			CaseID:       c.ID.String(),
			EvidenceType: string(item.Type),
			EntityType:   item.EntityType,
			EntityID:     item.EntityID,
			PayloadJson:  payload,
			CreatedAt:    formatStoredTime(item.CreatedAt),
		}); err != nil {
			return fmt.Errorf("create leakage evidence: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit leakage case transaction: %w", err)
	}

	committed = true
	return nil
}

func expectedRevenueEntryFromRow(row sqlitedb.ExpectedRevenueEntry) (revenue.ExpectedRevenueEntry, error) {
	id, err := parseStoredUUID("expected revenue entry id", row.ID)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, err
	}

	tenantID, err := parseStoredUUID("tenant id", row.TenantID)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, err
	}

	customerID, err := parseStoredUUID("customer id", row.CustomerID)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, err
	}

	contractID, err := parseStoredUUID("contract id", row.ContractID)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, err
	}

	billableItemID, err := parseStoredUUID("billable item id", row.BillableItemID)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, err
	}

	period, err := billingPeriodFromStoredValues(row.PeriodStart, row.PeriodEnd)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, err
	}

	expectedAmount, err := valueobject.NewMoney(row.Currency, row.ExpectedAmountMinorUnits)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, fmt.Errorf("build expected amount: %w", err)
	}

	calculationBasis, err := decodeJSONMap("calculation basis", row.CalculationBasisJson)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, err
	}

	calculatedAt, err := parseStoredTime("calculated at", row.CalculatedAt)
	if err != nil {
		return revenue.ExpectedRevenueEntry{}, err
	}

	return revenue.ExpectedRevenueEntry{
		ID:               id,
		TenantID:         tenantID,
		CustomerID:       customerID,
		ContractID:       contractID,
		BillableItemID:   billableItemID,
		Period:           period,
		ExpectedAmount:   expectedAmount,
		CalculationBasis: calculationBasis,
		CalculatedAt:     calculatedAt,
		Version:          int(row.Version),
		TraceID:          row.TraceID,
	}, nil
}

func actualRevenueEntryFromRow(row sqlitedb.ActualRevenueEntry) (revenue.ActualRevenueEntry, error) {
	id, err := parseStoredUUID("actual revenue entry id", row.ID)
	if err != nil {
		return revenue.ActualRevenueEntry{}, err
	}

	tenantID, err := parseStoredUUID("tenant id", row.TenantID)
	if err != nil {
		return revenue.ActualRevenueEntry{}, err
	}

	customerID, err := parseStoredUUID("customer id", row.CustomerID)
	if err != nil {
		return revenue.ActualRevenueEntry{}, err
	}

	contractID, err := parseStoredUUID("contract id", row.ContractID)
	if err != nil {
		return revenue.ActualRevenueEntry{}, err
	}

	billableItemID, err := parseStoredUUID("billable item id", row.BillableItemID)
	if err != nil {
		return revenue.ActualRevenueEntry{}, err
	}

	period, err := billingPeriodFromStoredValues(row.PeriodStart, row.PeriodEnd)
	if err != nil {
		return revenue.ActualRevenueEntry{}, err
	}

	actualAmount, err := valueobject.NewMoney(row.Currency, row.ActualAmountMinorUnits)
	if err != nil {
		return revenue.ActualRevenueEntry{}, fmt.Errorf("build actual amount: %w", err)
	}

	recognizedAt, err := parseStoredTime("recognized at", row.RecognizedAt)
	if err != nil {
		return revenue.ActualRevenueEntry{}, err
	}

	return revenue.ActualRevenueEntry{
		ID:             id,
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		Period:         period,
		ActualAmount:   actualAmount,
		RecognizedFrom: revenue.RecognizedFrom(row.RecognizedFrom),
		RecognizedAt:   recognizedAt,
		TraceID:        row.TraceID,
	}, nil
}

func billingPeriodFromStoredValues(startValue, endValue string) (valueobject.BillingPeriod, error) {
	start, err := parseStoredTime("period start", startValue)
	if err != nil {
		return valueobject.BillingPeriod{}, err
	}

	end, err := parseStoredTime("period end", endValue)
	if err != nil {
		return valueobject.BillingPeriod{}, err
	}

	period := valueobject.BillingPeriod{Start: start, End: end}
	if err := period.Validate(); err != nil {
		return valueobject.BillingPeriod{}, fmt.Errorf("build billing period: %w", err)
	}

	return period, nil
}

func parseStoredUUID(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse %s: %w", field, err)
	}

	return id, nil
}

func parseStoredTime(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", field, err)
	}

	return parsed, nil
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

func nullableUUIDInterface(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}

	return value.String()
}

func reconciliationRunSummaryFromRow(row sqlitedb.ReconciliationRun) (ReconciliationRunSummary, error) {
	id, err := parseStoredUUID("reconciliation run id", row.ID)
	if err != nil {
		return ReconciliationRunSummary{}, err
	}

	tenantID, err := parseStoredUUID("tenant id", row.TenantID)
	if err != nil {
		return ReconciliationRunSummary{}, err
	}

	contractID, err := parseStoredUUID("contract id", row.ContractID)
	if err != nil {
		return ReconciliationRunSummary{}, err
	}

	period, err := billingPeriodFromStoredValues(row.PeriodStart, row.PeriodEnd)
	if err != nil {
		return ReconciliationRunSummary{}, err
	}

	startedAt, err := parseStoredTime("started at", row.StartedAt)
	if err != nil {
		return ReconciliationRunSummary{}, err
	}

	var completedAt time.Time
	if row.CompletedAt.Valid && row.CompletedAt.String != "" {
		completedAt, err = parseStoredTime("completed at", row.CompletedAt.String)
		if err != nil {
			return ReconciliationRunSummary{}, err
		}
	}

	leakageAmount, err := valueobject.NewMoney(row.Currency, row.LeakageAmountMinorUnits)
	if err != nil {
		return ReconciliationRunSummary{}, fmt.Errorf("build leakage amount: %w", err)
	}

	return ReconciliationRunSummary{
		ID:            id,
		TenantID:      tenantID,
		ContractID:    contractID,
		Period:        period,
		Status:        ReconciliationRunStatus(row.Status),
		StartedAt:     startedAt.UTC(),
		CompletedAt:   completedAt.UTC(),
		ExpectedCount: row.ExpectedCount,
		ActualCount:   row.ActualCount,
		DiffCount:     row.DiffCount,
		CaseCount:     row.CaseCount,
		LeakageAmount: leakageAmount,
		TraceID:       row.TraceID,
	}, nil
}

func rollbackUncommitted(tx *sql.Tx, committed *bool) {
	if *committed {
		return
	}

	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return
	}
}

func decodeJSONMap(field, value string) (map[string]any, error) {
	if value == "" {
		return map[string]any{}, nil
	}

	decoded := make(map[string]any)
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil, fmt.Errorf("decode %s json: %w", field, err)
	}

	return decoded, nil
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
