package contract

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	contractdomain "github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/deplagene/revenueleakageengine/internal/migrator"
	"github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	"github.com/google/uuid"
)

func TestSQLiteStoreUpsertAndListEffectiveTerms(t *testing.T) {
	ctx := context.Background()
	store, db := newMigratedSQLiteStore(t, ctx)

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	customerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	contractID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	insertTenantAndCustomer(t, ctx, db, tenantID, customerID)

	contract := &contractdomain.Contract{
		ID:           contractID,
		TenantID:     tenantID,
		CustomerID:   customerID,
		Status:       contractdomain.StatusActive,
		StartDate:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Currency:     "USD",
		Version:      1,
		BillingModel: contractdomain.BillingModelFixedRecurring,
		Metadata: map[string]any{
			"source": "test",
		},
	}
	if err := store.UpsertContract(ctx, contract); err != nil {
		t.Fatalf("UpsertContract() error = %v", err)
	}

	terms := []*contractdomain.Term{
		{
			ID:            uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			TenantID:      tenantID,
			ContractID:    contractID,
			Type:          contractdomain.TermTypeFixedFee,
			EffectiveFrom: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			Priority:      10,
			Expression: map[string]any{
				"code":               "platform_subscription",
				"amount_minor_units": float64(50000),
				"currency":           "USD",
			},
		},
		{
			ID:            uuid.MustParse("55555555-5555-5555-5555-555555555555"),
			TenantID:      tenantID,
			ContractID:    contractID,
			Type:          contractdomain.TermTypeUsageRate,
			EffectiveFrom: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
			Priority:      20,
			Expression: map[string]any{
				"code":                   "api_calls",
				"unit_price_minor_units": float64(1),
				"included_quantity":      float64(100000),
				"currency":               "USD",
			},
		},
		{
			ID:            uuid.MustParse("66666666-6666-6666-6666-666666666666"),
			TenantID:      tenantID,
			ContractID:    contractID,
			Type:          contractdomain.TermTypeDiscount,
			EffectiveFrom: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EffectiveTo:   timePtr(time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)),
			Priority:      30,
			Expression: map[string]any{
				"percent_basis_points": float64(1000),
			},
		},
	}

	for _, term := range terms {
		if err := store.UpsertTerm(ctx, term); err != nil {
			t.Fatalf("UpsertTerm(%s) error = %v", term.ID, err)
		}
	}

	effectiveTerms, err := store.ListEffectiveTerms(
		ctx,
		contractID,
		time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("ListEffectiveTerms() error = %v", err)
	}

	if got := len(effectiveTerms); got != 2 {
		t.Fatalf("effective term count = %d, want 2", got)
	}

	if effectiveTerms[0].Type != contractdomain.TermTypeUsageRate {
		t.Fatalf("first effective term type = %s, want %s", effectiveTerms[0].Type, contractdomain.TermTypeUsageRate)
	}

	if effectiveTerms[1].Type != contractdomain.TermTypeFixedFee {
		t.Fatalf("second effective term type = %s, want %s", effectiveTerms[1].Type, contractdomain.TermTypeFixedFee)
	}
}

func TestSQLiteStoreGetContractNotFound(t *testing.T) {
	ctx := context.Background()
	store, _ := newMigratedSQLiteStore(t, ctx)

	_, err := store.GetContract(ctx, uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	if !errors.Is(err, ErrContractNotFound) {
		t.Fatalf("GetContract() error = %v, want %v", err, ErrContractNotFound)
	}
}

func newMigratedSQLiteStore(t *testing.T, ctx context.Context) (*SQLiteStore, *sql.DB) {
	t.Helper()

	db, err := sqlite.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	migrationsPath := filepath.Join("..", "..", "platform", "sqlite", "migrations")
	m := migrator.NewMigrator(db, migrationsPath)
	if err := m.Up(ctx); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return NewSQLiteStore(db), db
}

func insertTenantAndCustomer(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	tenantID uuid.UUID,
	customerID uuid.UUID,
) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO tenants (id, name, currency, timezone, created_at) VALUES (?, ?, ?, ?, ?)`,
		tenantID.String(),
		"Test Tenant",
		"USD",
		"UTC",
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO customer_accounts (id, tenant_id, name, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		customerID.String(),
		tenantID.String(),
		"Test Customer",
		"active",
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert customer: %v", err)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
