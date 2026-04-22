package reconciliation

import (
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

func TestMatcherMatchAndDiff(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	itemID := uuid.New()
	period := mustPeriod(t)

	matcher := NewMatcher()
	matches, err := matcher.Match(
		[]revenue.ExpectedRevenueEntry{
			{
				TenantID:       tenantID,
				CustomerID:     customerID,
				ContractID:     contractID,
				BillableItemID: itemID,
				Period:         period,
				ExpectedAmount: valueobject.MustMoney("USD", 5_800_00),
			},
		},
		[]revenue.ActualRevenueEntry{
			{
				TenantID:       tenantID,
				CustomerID:     customerID,
				ContractID:     contractID,
				BillableItemID: itemID,
				Period:         period,
				ActualAmount:   valueobject.MustMoney("USD", 4_640_00),
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected match error: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	diff, err := matches[0].Diff()
	if err != nil {
		t.Fatalf("unexpected diff error: %v", err)
	}

	if diff.LeakageAmount.MinorUnits != 1_160_00 {
		t.Fatalf("unexpected leakage amount: %d", diff.LeakageAmount.MinorUnits)
	}
}

func mustPeriod(t *testing.T) valueobject.BillingPeriod {
	t.Helper()

	period, err := valueobject.NewBillingPeriod(
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("unexpected period error: %v", err)
	}

	return period
}
