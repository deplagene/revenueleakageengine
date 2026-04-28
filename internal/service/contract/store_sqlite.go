package contract

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	contractdomain "github.com/deplagene/revenueleakageengine/internal/domain/contract"
	sqlitedb "github.com/deplagene/revenueleakageengine/internal/platform/sqlite/sqlc"
	"github.com/google/uuid"
)

var _ Store = (*SQLiteStore)(nil)

// SQLiteStore loads and persists contracts and terms using SQLite.
type SQLiteStore struct {
	queries *sqlitedb.Queries
}

// NewSQLiteStore creates a contract store backed by a SQLite database.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{
		queries: sqlitedb.New(db),
	}
}

func (s *SQLiteStore) GetContract(ctx context.Context, id uuid.UUID) (*contractdomain.Contract, error) {
	row, err := s.queries.GetContract(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrContractNotFound
		}

		return nil, fmt.Errorf("get contract: %w", err)
	}
	return contractFromRow(row)
}

func (s *SQLiteStore) UpsertContract(ctx context.Context, c *contractdomain.Contract) error {
	metadata, err := encodeJSONMap("contract metadata", c.Metadata)
	if err != nil {
		return err
	}

	var endDate sql.NullString
	if c.EndDate != nil {
		endDate = nullableStoredTime(c.EndDate)
	}

	var signedAt sql.NullString
	if c.SignedAt != nil {
		signedAt = nullableStoredTime(c.SignedAt)
	}

	now := formatStoredTime(time.Now())
	err = s.queries.UpsertContract(ctx, sqlitedb.UpsertContractParams{
		ID:           c.ID.String(),
		TenantID:     c.TenantID.String(),
		CustomerID:   c.CustomerID.String(),
		ExternalID:   sql.NullString{String: c.ExternalID, Valid: c.ExternalID != ""},
		Status:       string(c.Status),
		StartDate:    c.StartDate.UTC().Format(time.RFC3339Nano),
		EndDate:      endDate,
		Currency:     c.Currency,
		Version:      int64(c.Version),
		BillingModel: string(c.BillingModel),
		SignedAt:     signedAt,
		MetadataJson: metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return fmt.Errorf("upsert contract: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListContractsByCustomer(
	ctx context.Context,
	tenantID uuid.UUID,
	customerID uuid.UUID,
) ([]*contractdomain.Contract, error) {
	rows, err := s.queries.ListContractsByCustomer(ctx, sqlitedb.ListContractsByCustomerParams{
		TenantID:   tenantID.String(),
		CustomerID: customerID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list contracts: %w", err)
	}

	contracts := make([]*contractdomain.Contract, 0, len(rows))
	for _, row := range rows {
		c, err := contractFromRow(row)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, c)
	}
	return contracts, nil
}

func (s *SQLiteStore) GetBillableItem(ctx context.Context, id uuid.UUID) (*contractdomain.BillableItem, error) {
	row, err := s.queries.GetBillableItem(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBillableItemNotFound
		}

		return nil, fmt.Errorf("get billable item: %w", err)
	}
	return billableItemFromRow(row)
}

func (s *SQLiteStore) GetBillableItemByCode(
	ctx context.Context,
	tenantID uuid.UUID,
	code string,
) (*contractdomain.BillableItem, error) {
	row, err := s.queries.GetBillableItemByCode(ctx, sqlitedb.GetBillableItemByCodeParams{
		TenantID: tenantID.String(),
		Code:     code,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBillableItemNotFound
		}

		return nil, fmt.Errorf("get billable item by code: %w", err)
	}
	return billableItemFromRow(row)
}

func (s *SQLiteStore) UpsertBillableItem(ctx context.Context, item *contractdomain.BillableItem) error {
	err := s.queries.UpsertBillableItem(ctx, sqlitedb.UpsertBillableItemParams{
		ID:          item.ID.String(),
		TenantID:    item.TenantID.String(),
		Code:        item.Code,
		Name:        item.Name,
		Category:    item.Category,
		Unit:        item.Unit,
		PricingMode: string(item.PricingMode),
		Status:      string(item.Status),
		CreatedAt:   formatStoredTime(time.Now()),
	})
	if err != nil {
		return fmt.Errorf("upsert billable item: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListBillableItems(
	ctx context.Context,
	tenantID uuid.UUID,
) ([]*contractdomain.BillableItem, error) {
	rows, err := s.queries.ListBillableItems(ctx, tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("list billable items: %w", err)
	}

	items := make([]*contractdomain.BillableItem, 0, len(rows))
	for _, row := range rows {
		item, err := billableItemFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *SQLiteStore) UpsertTerm(ctx context.Context, t *contractdomain.Term) error {
	expression, err := encodeJSONMap("term expression", t.Expression)
	if err != nil {
		return err
	}

	var effectiveTo sql.NullString
	if t.EffectiveTo != nil {
		effectiveTo = nullableStoredTime(t.EffectiveTo)
	}

	err = s.queries.UpsertContractTerm(ctx, sqlitedb.UpsertContractTermParams{
		ID:             t.ID.String(),
		TenantID:       t.TenantID.String(),
		ContractID:     t.ContractID.String(),
		TermType:       string(t.Type),
		EffectiveFrom:  formatStoredTime(t.EffectiveFrom),
		EffectiveTo:    effectiveTo,
		Priority:       int64(t.Priority),
		ExpressionJson: expression,
		SourceRef:      t.SourceRef,
		CreatedAt:      formatStoredTime(time.Now()),
	})
	if err != nil {
		return fmt.Errorf("upsert term: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListEffectiveTerms(
	ctx context.Context,
	contractID uuid.UUID,
	at time.Time,
) ([]*contractdomain.Term, error) {
	atStr := formatStoredTime(at)
	rows, err := s.queries.ListEffectiveContractTerms(ctx, sqlitedb.ListEffectiveContractTermsParams{
		ContractID:    contractID.String(),
		EffectiveFrom: atStr,
		EffectiveTo:   sql.NullString{String: atStr, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list effective terms: %w", err)
	}

	terms := make([]*contractdomain.Term, 0, len(rows))
	for _, row := range rows {
		t, err := termFromRow(row)
		if err != nil {
			return nil, err
		}
		terms = append(terms, t)
	}
	return terms, nil
}

func contractFromRow(row sqlitedb.Contract) (*contractdomain.Contract, error) {
	id, err := parseStoredUUID("contract id", row.ID)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseStoredUUID("tenant id", row.TenantID)
	if err != nil {
		return nil, err
	}

	customerID, err := parseStoredUUID("customer id", row.CustomerID)
	if err != nil {
		return nil, err
	}

	startDate, err := parseStoredTime("contract start date", row.StartDate)
	if err != nil {
		return nil, err
	}

	var endDate *time.Time
	if row.EndDate.Valid {
		parsedEndDate, err := parseStoredTime("contract end date", row.EndDate.String)
		if err != nil {
			return nil, err
		}

		endDate = &parsedEndDate
	}

	var signedAt *time.Time
	if row.SignedAt.Valid {
		parsedSignedAt, err := parseStoredTime("contract signed at", row.SignedAt.String)
		if err != nil {
			return nil, err
		}

		signedAt = &parsedSignedAt
	}

	metadata, err := decodeJSONMap("contract metadata", row.MetadataJson)
	if err != nil {
		return nil, err
	}

	c := contractdomain.Contract{
		ID:           id,
		TenantID:     tenantID,
		CustomerID:   customerID,
		ExternalID:   row.ExternalID.String,
		Status:       contractdomain.Status(row.Status),
		StartDate:    startDate.UTC(),
		EndDate:      endDate,
		Currency:     row.Currency,
		Version:      int(row.Version),
		BillingModel: contractdomain.BillingModel(row.BillingModel),
		SignedAt:     signedAt,
		Metadata:     metadata,
	}.Normalize()

	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("build contract from row: %w", err)
	}

	return &c, nil
}

func billableItemFromRow(row sqlitedb.BillableItem) (*contractdomain.BillableItem, error) {
	id, err := parseStoredUUID("billable item id", row.ID)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseStoredUUID("tenant id", row.TenantID)
	if err != nil {
		return nil, err
	}

	item := contractdomain.BillableItem{
		ID:          id,
		TenantID:    tenantID,
		Code:        row.Code,
		Name:        row.Name,
		Category:    row.Category,
		Unit:        row.Unit,
		PricingMode: contractdomain.PricingMode(row.PricingMode),
		Status:      contractdomain.BillableItemStatus(row.Status),
	}.Normalize()

	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("build billable item from row: %w", err)
	}

	return &item, nil
}

func termFromRow(row sqlitedb.ContractTerm) (*contractdomain.Term, error) {
	id, err := parseStoredUUID("term id", row.ID)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseStoredUUID("tenant id", row.TenantID)
	if err != nil {
		return nil, err
	}

	contractID, err := parseStoredUUID("contract id", row.ContractID)
	if err != nil {
		return nil, err
	}

	effectiveFrom, err := parseStoredTime("term effective from", row.EffectiveFrom)
	if err != nil {
		return nil, err
	}

	var effectiveTo *time.Time
	if row.EffectiveTo.Valid {
		parsedEffectiveTo, err := parseStoredTime("term effective to", row.EffectiveTo.String)
		if err != nil {
			return nil, err
		}

		effectiveTo = &parsedEffectiveTo
	}

	expression, err := decodeJSONMap("term expression", row.ExpressionJson)
	if err != nil {
		return nil, err
	}

	term := contractdomain.Term{
		ID:            id,
		TenantID:      tenantID,
		ContractID:    contractID,
		Type:          contractdomain.TermType(row.TermType),
		EffectiveFrom: effectiveFrom.UTC(),
		EffectiveTo:   effectiveTo,
		Priority:      int(row.Priority),
		Expression:    expression,
		SourceRef:     row.SourceRef,
	}.Normalize()

	if err := term.Validate(); err != nil {
		return nil, fmt.Errorf("build term from row: %w", err)
	}

	return &term, nil
}

func parseStoredUUID(field string, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse %s: %w", field, err)
	}

	return id, nil
}

func parseStoredTime(field string, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", field, err)
	}

	return parsed.UTC(), nil
}

func formatStoredTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableStoredTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}

	return sql.NullString{
		String: formatStoredTime(*value),
		Valid:  true,
	}
}

func decodeJSONMap(field string, value string) (map[string]any, error) {
	if value == "" {
		return map[string]any{}, nil
	}

	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil, fmt.Errorf("decode %s json: %w", field, err)
	}

	if decoded == nil {
		return map[string]any{}, nil
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
