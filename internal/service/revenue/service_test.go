package revenue

import (
	"context"
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

func TestServiceCalculateExpectedRevenueFillsGeneratedFields(t *testing.T) {
	now := mustTime(t, "2026-05-01T01:00:00Z")
	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()

	service := NewService(WithClock(func() time.Time {
		return now
	}))

	entry, err := service.CalculateExpectedRevenue(context.Background(), CalculateExpectedRevenueCommand{
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		Period:         mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z"),
		Pricing: FixedUsagePricing{
			BaseFee:          valueobject.MustMoney("USD", 500_000),
			IncludedQuantity: 100_000,
			OverageUnitPrice: valueobject.MustMoney("USD", 1),
			Unit:             "events",
		},
		Version: 1,
	})
	if err != nil {
		t.Fatalf("CalculateExpectedRevenue() error = %v", err)
	}

	if entry.ID == uuid.Nil {
		t.Fatal("entry id was not generated")
	}

	if !entry.CalculatedAt.Equal(now) {
		t.Fatalf("calculated_at = %s, want %s", entry.CalculatedAt, now)
	}
}

func TestServiceCalculateAndSaveExpectedRevenue(t *testing.T) {
	now := mustTime(t, "2026-05-01T01:00:00Z")
	store := &fakeStore{}
	service := NewService(
		WithClock(func() time.Time {
			return now
		}),
		WithStore(store),
	)

	entry, err := service.CalculateAndSaveExpectedRevenue(context.Background(), CalculateExpectedRevenueCommand{
		TenantID:       uuid.New(),
		CustomerID:     uuid.New(),
		ContractID:     uuid.New(),
		BillableItemID: uuid.New(),
		Period:         mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z"),
		Pricing: FixedUsagePricing{
			BaseFee:          valueobject.MustMoney("USD", 500_000),
			IncludedQuantity: 100_000,
			OverageUnitPrice: valueobject.MustMoney("USD", 1),
			Unit:             "events",
		},
		Version: 1,
	})
	if err != nil {
		t.Fatalf("CalculateAndSaveExpectedRevenue() error = %v", err)
	}

	if store.savedExpectedCount != 1 {
		t.Fatalf("saved expected count = %d, want 1", store.savedExpectedCount)
	}

	if store.lastExpectedEntry.ID != entry.ID {
		t.Fatalf("stored entry id = %s, want %s", store.lastExpectedEntry.ID, entry.ID)
	}
}

func TestServiceBuildAndSaveActualRevenue(t *testing.T) {
	now := mustTime(t, "2026-04-30T23:00:00Z")
	store := &fakeStore{}
	service := NewService(
		WithClock(func() time.Time {
			return now
		}),
		WithStore(store),
	)

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	invoiceID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")

	entries, err := service.BuildAndSaveActualRevenue(context.Background(), BuildActualRevenueCommand{
		TenantID:   tenantID,
		CustomerID: customerID,
		ContractID: contractID,
		Period:     period,
		Invoices: []billing.Invoice{
			issuedInvoice(invoiceID, tenantID, customerID, contractID, period),
		},
		InvoiceLines: []billing.InvoiceLine{
			invoiceLine(invoiceID, billableItemID, valueobject.MustMoney("USD", 464_000)),
		},
	})
	if err != nil {
		t.Fatalf("BuildAndSaveActualRevenue() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}

	if store.savedActualCount != 1 {
		t.Fatalf("saved actual count = %d, want 1", store.savedActualCount)
	}

	if store.lastActualEntry.ID != entries[0].ID {
		t.Fatalf("stored actual entry id = %s, want %s", store.lastActualEntry.ID, entries[0].ID)
	}
}

type fakeStore struct {
	savedExpectedCount int
	savedActualCount   int
	lastExpectedEntry  revenuedomain.ExpectedRevenueEntry
	lastActualEntry    revenuedomain.ActualRevenueEntry
}

func (s *fakeStore) SaveExpectedRevenue(
	ctx context.Context,
	entry revenuedomain.ExpectedRevenueEntry,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.savedExpectedCount++
	s.lastExpectedEntry = entry
	return nil
}

func (s *fakeStore) SaveActualRevenue(
	ctx context.Context,
	entry revenuedomain.ActualRevenueEntry,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.savedActualCount++
	s.lastActualEntry = entry
	return nil
}
