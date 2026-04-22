package revenue

import (
	"errors"
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

func TestFixedUsageCalculatorCalculateExpectedRevenue(t *testing.T) {
	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")
	calculatedAt := mustTime(t, "2026-05-01T01:00:00Z")

	calculator := NewFixedUsageCalculator()

	entry, err := calculator.CalculateExpectedRevenue(CalculateExpectedRevenueCommand{
		EntryID:        uuid.New(),
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		Period:         period,
		Pricing: FixedUsagePricing{
			BaseFee:          valueobject.MustMoney("USD", 500_000),
			IncludedQuantity: 100_000,
			OverageUnitPrice: valueobject.MustMoney("USD", 1),
			Unit:             "events",
		},
		UsageRecords: []billing.UsageRecord{
			usageRecord(tenantID, customerID, contractID, billableItemID, "events", 100_000, "2026-04-10T00:00:00Z"),
			usageRecord(tenantID, customerID, contractID, billableItemID, "events", 80_000, "2026-04-20T00:00:00Z"),
		},
		CalculatedAt: calculatedAt,
		Version:      1,
		TraceID:      "trace-1",
	})
	if err != nil {
		t.Fatalf("CalculateExpectedRevenue() error = %v", err)
	}

	if entry.ExpectedAmount.MinorUnits != 580_000 {
		t.Fatalf("expected amount = %d, want 580000", entry.ExpectedAmount.MinorUnits)
	}

	if entry.ExpectedAmount.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", entry.ExpectedAmount.Currency)
	}

	assertBasisInt(t, entry.CalculationBasis, "base_fee_minor_units", 500_000)
	assertBasisInt(t, entry.CalculationBasis, "included_quantity", 100_000)
	assertBasisInt(t, entry.CalculationBasis, "usage_quantity", 180_000)
	assertBasisInt(t, entry.CalculationBasis, "billable_quantity", 80_000)
	assertBasisInt(t, entry.CalculationBasis, "overage_unit_price_minor_units", 1)
	assertBasisInt(t, entry.CalculationBasis, "overage_amount_minor_units", 80_000)
}

func TestFixedUsageCalculatorDoesNotChargeOverageBelowIncludedQuantity(t *testing.T) {
	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")

	calculator := NewFixedUsageCalculator()

	entry, err := calculator.CalculateExpectedRevenue(CalculateExpectedRevenueCommand{
		EntryID:        uuid.New(),
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		Period:         period,
		Pricing: FixedUsagePricing{
			BaseFee:          valueobject.MustMoney("USD", 500_000),
			IncludedQuantity: 100_000,
			OverageUnitPrice: valueobject.MustMoney("USD", 1),
			Unit:             "events",
		},
		UsageRecords: []billing.UsageRecord{
			usageRecord(tenantID, customerID, contractID, billableItemID, "events", 90_000, "2026-04-10T00:00:00Z"),
		},
		CalculatedAt: time.Now(),
		Version:      1,
	})
	if err != nil {
		t.Fatalf("CalculateExpectedRevenue() error = %v", err)
	}

	if entry.ExpectedAmount.MinorUnits != 500_000 {
		t.Fatalf("expected amount = %d, want 500000", entry.ExpectedAmount.MinorUnits)
	}

	assertBasisInt(t, entry.CalculationBasis, "billable_quantity", 0)
	assertBasisInt(t, entry.CalculationBasis, "overage_amount_minor_units", 0)
}

func TestFixedUsageCalculatorRejectsInvalidUsageRecords(t *testing.T) {
	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")

	tests := []struct {
		name    string
		record  billing.UsageRecord
		wantErr error
	}{
		{
			name: "mismatched unit",
			record: usageRecord(
				tenantID,
				customerID,
				contractID,
				billableItemID,
				"api_calls",
				10,
				"2026-04-10T00:00:00Z",
			),
			wantErr: ErrUsageRecordMismatch,
		},
		{
			name: "outside period",
			record: usageRecord(
				tenantID,
				customerID,
				contractID,
				billableItemID,
				"events",
				10,
				"2026-05-10T00:00:00Z",
			),
			wantErr: ErrUsageRecordOutOfPeriod,
		},
		{
			name: "negative quantity",
			record: usageRecord(
				tenantID,
				customerID,
				contractID,
				billableItemID,
				"events",
				-10,
				"2026-04-10T00:00:00Z",
			),
			wantErr: ErrUsageQuantityInvalid,
		},
	}

	calculator := NewFixedUsageCalculator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := calculator.CalculateExpectedRevenue(CalculateExpectedRevenueCommand{
				EntryID:        uuid.New(),
				TenantID:       tenantID,
				CustomerID:     customerID,
				ContractID:     contractID,
				BillableItemID: billableItemID,
				Period:         period,
				Pricing: FixedUsagePricing{
					BaseFee:          valueobject.MustMoney("USD", 500_000),
					IncludedQuantity: 100_000,
					OverageUnitPrice: valueobject.MustMoney("USD", 1),
					Unit:             "events",
				},
				UsageRecords: []billing.UsageRecord{tt.record},
				CalculatedAt: time.Now(),
				Version:      1,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func assertBasisInt(t *testing.T, basis map[string]any, key string, want int64) {
	t.Helper()

	got, ok := basis[key].(int64)
	if !ok {
		t.Fatalf("basis[%q] has type %T, want int64", key, basis[key])
	}

	if got != want {
		t.Fatalf("basis[%q] = %d, want %d", key, got, want)
	}
}

func usageRecord(
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	billableItemID uuid.UUID,
	unit string,
	quantity int64,
	usageTime string,
) billing.UsageRecord {
	return billing.UsageRecord{
		ID:             uuid.New(),
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		UsageTime:      mustTimeNoT(usageTime),
		Quantity:       quantity,
		Unit:           unit,
	}
}

func mustPeriod(t *testing.T, start, end string) valueobject.BillingPeriod {
	t.Helper()

	period, err := valueobject.NewBillingPeriod(mustTime(t, start), mustTime(t, end))
	if err != nil {
		t.Fatalf("NewBillingPeriod() error = %v", err)
	}

	return period
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", value, err)
	}

	return parsed
}

func mustTimeNoT(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}

	return parsed
}
