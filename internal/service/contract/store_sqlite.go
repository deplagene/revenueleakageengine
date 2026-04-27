package contract

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/contract"
	sqlitedb "github.com/deplagene/revenueleakageengine/internal/platform/sqlite/sqlc"
	"github.com/google/uuid"
)

var _ Store = (*SQLiteStore)(nil)

// SQLiteStore loads and persists contracts and terms using SQLite.
type SQLiteStore struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// NewSQLiteStore creates a contract store backed by a SQLite database.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{
		db:      db,
		queries: sqlitedb.New(db),
	}
}

func (s *SQLiteStore) GetContract(ctx context.Context, id uuid.UUID) (*contract.Contract, error) {
	row, err := s.queries.GetContract(ctx, id.String())
	if err != nil {
		return nil, fmt.Errorf("get contract: %w", err)
	}
	return contractFromRow(row)
}

func (s *SQLiteStore) UpsertContract(ctx context.Context, c *contract.Contract) error {
	metadata, err := json.Marshal(c.Metadata)
	if err != nil {
		return fmt.Errorf("marshal contract metadata: %w", err)
	}

	var endDate sql.NullString
	if c.EndDate != nil {
		endDate = sql.NullString{String: c.EndDate.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	var signedAt sql.NullString
	if c.SignedAt != nil {
		signedAt = sql.NullString{String: c.SignedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

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
		MetadataJson: string(metadata),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("upsert contract: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListContractsByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]*contract.Contract, error) {
	rows, err := s.queries.ListContractsByCustomer(ctx, sqlitedb.ListContractsByCustomerParams{
		TenantID:   tenantID.String(),
		CustomerID: customerID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list contracts: %w", err)
	}

	contracts := make([]*contract.Contract, 0, len(rows))
	for _, row := range rows {
		c, err := contractFromRow(row)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, c)
	}
	return contracts, nil
}

func (s *SQLiteStore) GetBillableItem(ctx context.Context, id uuid.UUID) (*contract.BillableItem, error) {
	row, err := s.queries.GetBillableItem(ctx, id.String())
	if err != nil {
		return nil, fmt.Errorf("get billable item: %w", err)
	}
	return billableItemFromRow(row)
}

func (s *SQLiteStore) GetBillableItemByCode(ctx context.Context, tenantID uuid.UUID, code string) (*contract.BillableItem, error) {
	row, err := s.queries.GetBillableItemByCode(ctx, sqlitedb.GetBillableItemByCodeParams{
		TenantID: tenantID.String(),
		Code:     code,
	})
	if err != nil {
		return nil, fmt.Errorf("get billable item by code: %w", err)
	}
	return billableItemFromRow(row)
}

func (s *SQLiteStore) UpsertBillableItem(ctx context.Context, item *contract.BillableItem) error {
	err := s.queries.UpsertBillableItem(ctx, sqlitedb.UpsertBillableItemParams{
		ID:          item.ID.String(),
		TenantID:    item.TenantID.String(),
		Code:        item.Code,
		Name:        item.Name,
		Category:    item.Category,
		Unit:        item.Unit,
		PricingMode: string(item.PricingMode),
		Status:      string(item.Status),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("upsert billable item: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListBillableItems(ctx context.Context, tenantID uuid.UUID) ([]*contract.BillableItem, error) {
	rows, err := s.queries.ListBillableItems(ctx, tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("list billable items: %w", err)
	}

	items := make([]*contract.BillableItem, 0, len(rows))
	for _, row := range rows {
		item, err := billableItemFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *SQLiteStore) UpsertTerm(ctx context.Context, t *contract.Term) error {
	expression, err := json.Marshal(t.Expression)
	if err != nil {
		return fmt.Errorf("marshal term expression: %w", err)
	}

	var effectiveTo sql.NullString
	if t.EffectiveTo != nil {
		effectiveTo = sql.NullString{String: t.EffectiveTo.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	err = s.queries.UpsertContractTerm(ctx, sqlitedb.UpsertContractTermParams{
		ID:             t.ID.String(),
		TenantID:       t.TenantID.String(),
		ContractID:     t.ContractID.String(),
		TermType:       string(t.Type),
		EffectiveFrom:  t.EffectiveFrom.UTC().Format(time.RFC3339Nano),
		EffectiveTo:    effectiveTo,
		Priority:       int64(t.Priority),
		ExpressionJson: string(expression),
		SourceRef:      t.SourceRef,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("upsert term: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListEffectiveTerms(ctx context.Context, contractID uuid.UUID, at time.Time) ([]*contract.Term, error) {
	atStr := at.UTC().Format(time.RFC3339Nano)
	rows, err := s.queries.ListEffectiveContractTerms(ctx, sqlitedb.ListEffectiveContractTermsParams{
		ContractID:    contractID.String(),
		EffectiveFrom: atStr,
		EffectiveTo:   sql.NullString{String: atStr, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list effective terms: %w", err)
	}

	terms := make([]*contract.Term, 0, len(rows))
	for _, row := range rows {
		t, err := termFromRow(row)
		if err != nil {
			return nil, err
		}
		terms = append(terms, t)
	}
	return terms, nil
}

func contractFromRow(row sqlitedb.Contract) (*contract.Contract, error) {
	id, _ := uuid.Parse(row.ID)
	tenantID, _ := uuid.Parse(row.TenantID)
	customerID, _ := uuid.Parse(row.CustomerID)
	startDate, _ := time.Parse(time.RFC3339Nano, row.StartDate)

	var endDate *time.Time
	if row.EndDate.Valid {
		t, _ := time.Parse(time.RFC3339Nano, row.EndDate.String)
		endDate = &t
	}

	var signedAt *time.Time
	if row.SignedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, row.SignedAt.String)
		signedAt = &t
	}

	var metadata map[string]any
	json.Unmarshal([]byte(row.MetadataJson), &metadata)

	return &contract.Contract{
		ID:           id,
		TenantID:     tenantID,
		CustomerID:   customerID,
		ExternalID:   row.ExternalID.String,
		Status:       contract.Status(row.Status),
		StartDate:    startDate,
		EndDate:      endDate,
		Currency:     row.Currency,
		Version:      int(row.Version),
		BillingModel: contract.BillingModel(row.BillingModel),
		SignedAt:     signedAt,
		Metadata:     metadata,
	}, nil
}

func billableItemFromRow(row sqlitedb.BillableItem) (*contract.BillableItem, error) {
	id, _ := uuid.Parse(row.ID)
	tenantID, _ := uuid.Parse(row.TenantID)
	return &contract.BillableItem{
		ID:          id,
		TenantID:    tenantID,
		Code:        row.Code,
		Name:        row.Name,
		Category:    row.Category,
		Unit:        row.Unit,
		PricingMode: contract.PricingMode(row.PricingMode),
		Status:      contract.BillableItemStatus(row.Status),
	}, nil
}

func termFromRow(row sqlitedb.ContractTerm) (*contract.Term, error) {
	id, _ := uuid.Parse(row.ID)
	tenantID, _ := uuid.Parse(row.TenantID)
	contractID, _ := uuid.Parse(row.ContractID)
	effectiveFrom, _ := time.Parse(time.RFC3339Nano, row.EffectiveFrom)

	var effectiveTo *time.Time
	if row.EffectiveTo.Valid {
		t, _ := time.Parse(time.RFC3339Nano, row.EffectiveTo.String)
		effectiveTo = &t
	}

	var expression map[string]any
	json.Unmarshal([]byte(row.ExpressionJson), &expression)

	return &contract.Term{
		ID:            id,
		TenantID:      tenantID,
		ContractID:    contractID,
		Type:          contract.TermType(row.TermType),
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   effectiveTo,
		Priority:      int(row.Priority),
		Expression:    expression,
		SourceRef:     row.SourceRef,
	}, nil
}
