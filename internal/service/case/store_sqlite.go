package casework

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	sqlitedb "github.com/deplagene/revenueleakageengine/internal/platform/sqlite/sqlc"
	"github.com/google/uuid"
)

var _ Store = (*SQLiteStore)(nil)

// SQLiteStore loads leakage cases from SQLite-backed sqlc queries.
type SQLiteStore struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// NewSQLiteStore creates a case store backed by a SQLite database.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{
		db:      db,
		queries: sqlitedb.New(db),
	}
}

// ListCases returns one page of leakage cases for the requested tenant and
// optional filters.
func (s *SQLiteStore) ListCases(ctx context.Context, cmd ListCasesCommand) ([]leakage.Case, error) {
	rows, err := s.queries.ListLeakageCases(ctx, sqlitedb.ListLeakageCasesParams{
		TenantID:    cmd.TenantID.String(),
		ContractID:  nullableUUID(cmd.ContractID),
		Status:      nullableString(string(cmd.Status)),
		LimitCount:  int64(cmd.Limit),
		OffsetCount: int64(cmd.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list leakage cases: %w", err)
	}

	cases := make([]leakage.Case, 0, len(rows))
	for _, row := range rows {
		item, err := leakageCaseFromRow(row)
		if err != nil {
			return nil, err
		}

		cases = append(cases, item)
	}

	return cases, nil
}

// GetCase returns one detailed leakage case view together with its evidence
// and root-cause records.
func (s *SQLiteStore) GetCase(ctx context.Context, cmd GetCaseCommand) (GetCaseResult, error) {
	row, err := s.queries.GetLeakageCase(ctx, sqlitedb.GetLeakageCaseParams{
		TenantID: cmd.TenantID.String(),
		CaseID:   cmd.CaseID.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GetCaseResult{}, ErrCaseNotFound
		}

		return GetCaseResult{}, fmt.Errorf("get leakage case: %w", err)
	}

	c, err := leakageCaseFromRow(row)
	if err != nil {
		return GetCaseResult{}, err
	}

	evidenceRows, err := s.queries.ListLeakageEvidenceByCase(ctx, cmd.CaseID.String())
	if err != nil {
		return GetCaseResult{}, fmt.Errorf("list leakage evidence: %w", err)
	}

	evidence := make([]leakage.Evidence, 0, len(evidenceRows))
	for _, row := range evidenceRows {
		item, err := leakageEvidenceFromRow(row)
		if err != nil {
			return GetCaseResult{}, err
		}

		evidence = append(evidence, item)
	}

	rootCauseRows, err := s.queries.ListRootCausesByCase(ctx, cmd.CaseID.String())
	if err != nil {
		return GetCaseResult{}, fmt.Errorf("list root causes: %w", err)
	}

	rootCauses := make([]leakage.RootCause, 0, len(rootCauseRows))
	for _, row := range rootCauseRows {
		item, err := rootCauseFromRow(row)
		if err != nil {
			return GetCaseResult{}, err
		}

		rootCauses = append(rootCauses, item)
	}

	historyRows, err := s.queries.ListCaseStatusHistoryByCase(ctx, cmd.CaseID.String())
	if err != nil {
		return GetCaseResult{}, fmt.Errorf("list case status history: %w", err)
	}

	history := make([]leakage.StatusHistory, 0, len(historyRows))
	for _, row := range historyRows {
		item, err := statusHistoryFromRow(row)
		if err != nil {
			return GetCaseResult{}, err
		}

		history = append(history, item)
	}

	return GetCaseResult{
		Case:       c,
		Evidence:   evidence,
		RootCauses: rootCauses,
		History:    history,
	}, nil
}

// UpdateCaseStatus persists one status change together with its audit record.
func (s *SQLiteStore) UpdateCaseStatus(
	ctx context.Context,
	c leakage.Case,
	history leakage.StatusHistory,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin case status transaction: %w", err)
	}

	committed := false
	defer rollbackUncommitted(tx, &committed)

	queries := s.queries.WithTx(tx)
	updatedRows, err := queries.UpdateLeakageCaseStatus(ctx, sqlitedb.UpdateLeakageCaseStatusParams{
		Status:   string(c.Status),
		TenantID: c.TenantID.String(),
		CaseID:   c.ID.String(),
	})
	if err != nil {
		return fmt.Errorf("update leakage case status: %w", err)
	}

	if updatedRows == 0 {
		return ErrCaseNotFound
	}

	if err := queries.CreateCaseStatusHistory(ctx, sqlitedb.CreateCaseStatusHistoryParams{
		ID:         history.ID.String(),
		CaseID:     history.CaseID.String(),
		FromStatus: string(history.FromStatus),
		ToStatus:   string(history.ToStatus),
		ChangedAt:  history.ChangedAt.UTC().Format(time.RFC3339Nano),
		ChangedBy:  history.ChangedBy,
		ReasonCode: history.ReasonCode,
		Comment:    history.Comment,
	}); err != nil {
		return fmt.Errorf("create case status history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit case status transaction: %w", err)
	}

	committed = true
	return nil
}

// UpdateCaseAssignee persists one ownership change for a leakage case.
func (s *SQLiteStore) UpdateCaseAssignee(ctx context.Context, c leakage.Case) error {
	updatedRows, err := s.queries.UpdateLeakageCaseAssignee(ctx, sqlitedb.UpdateLeakageCaseAssigneeParams{
		Assignee: c.Assignee,
		TenantID: c.TenantID.String(),
		CaseID:   c.ID.String(),
	})
	if err != nil {
		return fmt.Errorf("update leakage case assignee: %w", err)
	}

	if updatedRows == 0 {
		return ErrCaseNotFound
	}

	return nil
}

func leakageCaseFromRow(row sqlitedb.LeakageCase) (leakage.Case, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return leakage.Case{}, fmt.Errorf("parse case id: %w", err)
	}

	tenantID, err := uuid.Parse(row.TenantID)
	if err != nil {
		return leakage.Case{}, fmt.Errorf("parse tenant id: %w", err)
	}

	customerID, err := uuid.Parse(row.CustomerID)
	if err != nil {
		return leakage.Case{}, fmt.Errorf("parse customer id: %w", err)
	}

	contractID, err := uuid.Parse(row.ContractID)
	if err != nil {
		return leakage.Case{}, fmt.Errorf("parse contract id: %w", err)
	}

	reconciliationRunID, err := parseNullableUUID("reconciliation run id", row.ReconciliationRunID)
	if err != nil {
		return leakage.Case{}, err
	}

	detectedAt, err := time.Parse(time.RFC3339Nano, row.DetectedAt)
	if err != nil {
		return leakage.Case{}, fmt.Errorf("parse detected at: %w", err)
	}

	period, err := billingPeriodFromStrings(row.PeriodStart, row.PeriodEnd)
	if err != nil {
		return leakage.Case{}, err
	}

	expectedAmount, err := valueobject.NewMoney(row.Currency, row.ExpectedAmountMinorUnits)
	if err != nil {
		return leakage.Case{}, fmt.Errorf("build expected amount: %w", err)
	}

	actualAmount, err := valueobject.NewMoney(row.Currency, row.ActualAmountMinorUnits)
	if err != nil {
		return leakage.Case{}, fmt.Errorf("build actual amount: %w", err)
	}

	leakageAmount, err := valueobject.NewMoney(row.Currency, row.LeakageAmountMinorUnits)
	if err != nil {
		return leakage.Case{}, fmt.Errorf("build leakage amount: %w", err)
	}

	if row.ConfidenceScoreBasisPoints < 0 ||
		row.ConfidenceScoreBasisPoints > int64(valueobject.MaxConfidenceBasisPoints) {
		return leakage.Case{}, fmt.Errorf(
			"build confidence score: basis points out of range: %d",
			row.ConfidenceScoreBasisPoints,
		)
	}

	confidenceScore, err := valueobject.NewConfidenceScore(uint16(row.ConfidenceScoreBasisPoints))
	if err != nil {
		return leakage.Case{}, fmt.Errorf("build confidence score: %w", err)
	}

	return leakage.Case{
		ID:                  id,
		TenantID:            tenantID,
		CustomerID:          customerID,
		ContractID:          contractID,
		ReconciliationRunID: reconciliationRunID,
		Type:                leakage.CaseType(row.CaseType),
		Severity:            leakage.Severity(row.Severity),
		Status:              leakage.Status(row.Status),
		DetectedAt:          detectedAt.UTC(),
		Period:              period,
		ExpectedAmount:      expectedAmount,
		ActualAmount:        actualAmount,
		LeakageAmount:       leakageAmount,
		ConfidenceScore:     confidenceScore,
		RootCauseCategory:   leakage.Category(row.RootCauseCategory),
		Assignee:            row.Assignee,
		TraceID:             row.TraceID,
	}, nil
}

func leakageEvidenceFromRow(row sqlitedb.LeakageEvidence) (leakage.Evidence, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return leakage.Evidence{}, fmt.Errorf("parse evidence id: %w", err)
	}

	caseID, err := uuid.Parse(row.CaseID)
	if err != nil {
		return leakage.Evidence{}, fmt.Errorf("parse evidence case id: %w", err)
	}

	payload, err := decodeJSONMap("evidence payload", row.PayloadJson)
	if err != nil {
		return leakage.Evidence{}, err
	}

	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return leakage.Evidence{}, fmt.Errorf("parse evidence created at: %w", err)
	}

	return leakage.Evidence{
		ID:         id,
		CaseID:     caseID,
		Type:       leakage.EvidenceType(row.EvidenceType),
		EntityType: row.EntityType,
		EntityID:   row.EntityID,
		Payload:    payload,
		CreatedAt:  createdAt.UTC(),
	}, nil
}

func rootCauseFromRow(row sqlitedb.RootCause) (leakage.RootCause, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return leakage.RootCause{}, fmt.Errorf("parse root cause id: %w", err)
	}

	caseID, err := uuid.Parse(row.CaseID)
	if err != nil {
		return leakage.RootCause{}, fmt.Errorf("parse root cause case id: %w", err)
	}

	if row.ConfidenceScoreBasisPoints < 0 ||
		row.ConfidenceScoreBasisPoints > int64(valueobject.MaxConfidenceBasisPoints) {
		return leakage.RootCause{}, fmt.Errorf(
			"build root cause confidence score: basis points out of range: %d",
			row.ConfidenceScoreBasisPoints,
		)
	}

	confidenceScore, err := valueobject.NewConfidenceScore(uint16(row.ConfidenceScoreBasisPoints))
	if err != nil {
		return leakage.RootCause{}, fmt.Errorf("build root cause confidence score: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return leakage.RootCause{}, fmt.Errorf("parse root cause created at: %w", err)
	}

	return leakage.RootCause{
		ID:              id,
		CaseID:          caseID,
		Category:        leakage.Category(row.Category),
		Subcategory:     row.Subcategory,
		Description:     row.Description,
		ConfidenceScore: confidenceScore,
		DerivedBy:       leakage.DerivedBy(row.DerivedBy),
		CreatedAt:       createdAt.UTC(),
	}, nil
}

func statusHistoryFromRow(row sqlitedb.CaseStatusHistory) (leakage.StatusHistory, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return leakage.StatusHistory{}, fmt.Errorf("parse case status history id: %w", err)
	}

	caseID, err := uuid.Parse(row.CaseID)
	if err != nil {
		return leakage.StatusHistory{}, fmt.Errorf("parse case status history case id: %w", err)
	}

	changedAt, err := time.Parse(time.RFC3339Nano, row.ChangedAt)
	if err != nil {
		return leakage.StatusHistory{}, fmt.Errorf("parse case status history changed at: %w", err)
	}

	return leakage.StatusHistory{
		ID:         id,
		CaseID:     caseID,
		FromStatus: leakage.Status(row.FromStatus),
		ToStatus:   leakage.Status(row.ToStatus),
		ChangedAt:  changedAt.UTC(),
		ChangedBy:  row.ChangedBy,
		ReasonCode: row.ReasonCode,
		Comment:    row.Comment,
	}, nil
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

func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}

	return sql.NullString{
		String: value,
		Valid:  true,
	}
}

func rollbackUncommitted(tx *sql.Tx, committed *bool) {
	if *committed {
		return
	}

	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return
	}
}

func parseNullableUUID(field string, value sql.NullString) (uuid.UUID, error) {
	if !value.Valid || value.String == "" {
		return uuid.Nil, nil
	}

	parsed, err := uuid.Parse(value.String)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse %s: %w", field, err)
	}

	return parsed, nil
}

func billingPeriodFromStrings(start, end string) (valueobject.BillingPeriod, error) {
	startTime, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return valueobject.BillingPeriod{}, fmt.Errorf("parse period start: %w", err)
	}

	endTime, err := time.Parse(time.RFC3339Nano, end)
	if err != nil {
		return valueobject.BillingPeriod{}, fmt.Errorf("parse period end: %w", err)
	}

	period, err := valueobject.NewBillingPeriod(startTime, endTime)
	if err != nil {
		return valueobject.BillingPeriod{}, fmt.Errorf("build billing period: %w", err)
	}

	return period, nil
}

func decodeJSONMap(field, raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("decode %s json: %w", field, err)
	}

	if payload == nil {
		return map[string]any{}, nil
	}

	return payload, nil
}
