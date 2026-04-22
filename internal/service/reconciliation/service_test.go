package reconciliation

import (
	"context"
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

func TestServiceReconcilePeriodCreatesUnderbillingCase(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	itemID := uuid.New()
	period := mustPeriod(t)

	store := &fakeStore{
		expected: []revenue.ExpectedRevenueEntry{
			{
				TenantID:       tenantID,
				CustomerID:     customerID,
				ContractID:     contractID,
				BillableItemID: itemID,
				Period:         period,
				ExpectedAmount: valueobject.MustMoney("USD", 5_800_00),
			},
		},
		actual: []revenue.ActualRevenueEntry{
			{
				TenantID:       tenantID,
				CustomerID:     customerID,
				ContractID:     contractID,
				BillableItemID: itemID,
				Period:         period,
				ActualAmount:   valueobject.MustMoney("USD", 4_640_00),
			},
		},
	}

	service, err := NewService(
		store,
		WithClock(func() time.Time {
			return time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC)
		}),
	)
	if err != nil {
		t.Fatalf("unexpected service error: %v", err)
	}

	result, err := service.ReconcilePeriod(context.Background(), ReconcilePeriodCommand{
		TenantID:             tenantID,
		ContractID:           contractID,
		Period:               period,
		RunID:                uuid.New(),
		TraceID:              "trace-1",
		Currency:             "USD",
		MinimumLeakageAmount: valueobject.ZeroMoney("USD"),
	})
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}

	if result.CaseCount != 1 {
		t.Fatalf("expected 1 case, got %d", result.CaseCount)
	}

	if result.LeakageAmount.MinorUnits != 1_160_00 {
		t.Fatalf("unexpected leakage amount: %d", result.LeakageAmount.MinorUnits)
	}

	if len(store.createdCases) != 1 {
		t.Fatalf("expected 1 persisted case, got %d", len(store.createdCases))
	}

	if store.createdCases[0].Type != leakage.CaseTypeUnderbilling {
		t.Fatalf("unexpected case type: %s", store.createdCases[0].Type)
	}

	if len(store.createdEvidence) != 1 {
		t.Fatalf("expected 1 evidence record, got %d", len(store.createdEvidence))
	}
}

func TestServiceReconcilePeriodCreatesUnbilledUsageCase(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	itemID := uuid.New()
	period := mustPeriod(t)

	store := &fakeStore{
		expected: []revenue.ExpectedRevenueEntry{
			{
				TenantID:       tenantID,
				CustomerID:     customerID,
				ContractID:     contractID,
				BillableItemID: itemID,
				Period:         period,
				ExpectedAmount: valueobject.MustMoney("USD", 1_000_00),
			},
		},
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("unexpected service error: %v", err)
	}

	result, err := service.ReconcilePeriod(context.Background(), ReconcilePeriodCommand{
		TenantID:             tenantID,
		ContractID:           contractID,
		Period:               period,
		RunID:                uuid.New(),
		Currency:             "USD",
		MinimumLeakageAmount: valueobject.ZeroMoney("USD"),
	})
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}

	if result.CaseCount != 1 {
		t.Fatalf("expected 1 case, got %d", result.CaseCount)
	}

	if store.createdCases[0].Type != leakage.CaseTypeUnbilledUsage {
		t.Fatalf("unexpected case type: %s", store.createdCases[0].Type)
	}
}

type fakeStore struct {
	expected        []revenue.ExpectedRevenueEntry
	actual          []revenue.ActualRevenueEntry
	createdCases    []leakage.Case
	createdEvidence []leakage.Evidence
}

func (f *fakeStore) ListExpectedRevenue(
	ctx context.Context,
	tenantID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) ([]revenue.ExpectedRevenueEntry, error) {
	return f.expected, nil
}

func (f *fakeStore) ListActualRevenue(
	ctx context.Context,
	tenantID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) ([]revenue.ActualRevenueEntry, error) {
	return f.actual, nil
}

func (f *fakeStore) CreateLeakageCase(
	ctx context.Context,
	c leakage.Case,
	evidence []leakage.Evidence,
) error {
	f.createdCases = append(f.createdCases, c)
	f.createdEvidence = append(f.createdEvidence, evidence...)

	return nil
}
