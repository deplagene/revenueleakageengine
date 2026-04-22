package reconciliation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	revenueservice "github.com/deplagene/revenueleakageengine/internal/service/revenue"
	"github.com/google/uuid"
)

func TestRevenueLeakageWorkflowRunRevenueLeakageCheck(t *testing.T) {
	ctx := context.Background()
	store := &workflowStore{
		expected: []revenuedomain.ExpectedRevenueEntry{},
		actual:   []revenuedomain.ActualRevenueEntry{},
		cases:    []leakage.Case{},
	}
	now := mustTime(t, "2026-05-01T01:00:00Z")

	revenueService := revenueservice.NewService(
		revenueservice.WithStore(store),
		revenueservice.WithClock(func() time.Time {
			return now
		}),
	)
	reconciliationService, err := reconciliationservice.NewService(
		store,
		reconciliationservice.WithClock(func() time.Time {
			return now
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	workflow, err := NewRevenueLeakageWorkflow(revenueService, reconciliationService)
	if err != nil {
		t.Fatalf("NewRevenueLeakageWorkflow() error = %v", err)
	}

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	invoiceID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")

	result, err := workflow.RunRevenueLeakageCheck(ctx, RunRevenueLeakageCheckCommand{
		Expected: revenueservice.CalculateExpectedRevenueCommand{
			TenantID:       tenantID,
			CustomerID:     customerID,
			ContractID:     contractID,
			BillableItemID: billableItemID,
			Period:         period,
			Pricing: revenueservice.FixedUsagePricing{
				BaseFee:          valueobject.MustMoney("USD", 500_000),
				IncludedQuantity: 100_000,
				OverageUnitPrice: valueobject.MustMoney("USD", 1),
				Unit:             "events",
			},
			UsageRecords: []billing.UsageRecord{
				usageRecord(tenantID, customerID, contractID, billableItemID, 180_000, "2026-04-10T00:00:00Z"),
			},
			Version: 1,
		},
		Actual: revenueservice.BuildActualRevenueCommand{
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
		},
		TraceID: "business-trace-1",
	})
	if err != nil {
		t.Fatalf("RunRevenueLeakageCheck() error = %v", err)
	}

	if result.ExpectedEntry == uuid.Nil {
		t.Fatal("expected entry id was not returned")
	}

	if result.ActualEntryCount != 1 {
		t.Fatalf("actual entry count = %d, want 1", result.ActualEntryCount)
	}

	if result.ReconciliationResult.CaseCount != 1 {
		t.Fatalf("case count = %d, want 1", result.ReconciliationResult.CaseCount)
	}

	if result.ReconciliationResult.LeakageAmount.MinorUnits != 116_000 {
		t.Fatalf(
			"leakage amount = %d, want 116000",
			result.ReconciliationResult.LeakageAmount.MinorUnits,
		)
	}

	if len(store.cases) != 1 {
		t.Fatalf("stored case count = %d, want 1", len(store.cases))
	}

	if store.cases[0].Type != leakage.CaseTypeUnderbilling {
		t.Fatalf("case type = %q, want underbilling", store.cases[0].Type)
	}
}

func TestRevenueLeakageWorkflowRejectsScopeMismatch(t *testing.T) {
	store := &workflowStore{
		expected: []revenuedomain.ExpectedRevenueEntry{},
		actual:   []revenuedomain.ActualRevenueEntry{},
		cases:    []leakage.Case{},
	}
	reconciliationService, err := reconciliationservice.NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	workflow, err := NewRevenueLeakageWorkflow(
		revenueservice.NewService(revenueservice.WithStore(store)),
		reconciliationService,
	)
	if err != nil {
		t.Fatalf("NewRevenueLeakageWorkflow() error = %v", err)
	}

	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")
	_, err = workflow.RunRevenueLeakageCheck(context.Background(), RunRevenueLeakageCheckCommand{
		Expected: revenueservice.CalculateExpectedRevenueCommand{
			TenantID:       uuid.New(),
			CustomerID:     uuid.New(),
			ContractID:     uuid.New(),
			BillableItemID: uuid.New(),
			Period:         period,
		},
		Actual: revenueservice.BuildActualRevenueCommand{
			TenantID:   uuid.New(),
			CustomerID: uuid.New(),
			ContractID: uuid.New(),
			Period:     period,
		},
	})
	if !errors.Is(err, ErrWorkflowScopeMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrWorkflowScopeMismatch)
	}
}

type workflowStore struct {
	expected []revenuedomain.ExpectedRevenueEntry
	actual   []revenuedomain.ActualRevenueEntry
	cases    []leakage.Case
}

func (s *workflowStore) SaveExpectedRevenue(
	ctx context.Context,
	entry revenuedomain.ExpectedRevenueEntry,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.expected = append(s.expected, entry)
	return nil
}

func (s *workflowStore) SaveActualRevenue(
	ctx context.Context,
	entry revenuedomain.ActualRevenueEntry,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.actual = append(s.actual, entry)
	return nil
}

func (s *workflowStore) ListExpectedRevenue(
	ctx context.Context,
	tenantID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) ([]revenuedomain.ExpectedRevenueEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := make([]revenuedomain.ExpectedRevenueEntry, 0, len(s.expected))
	for _, entry := range s.expected {
		if entry.TenantID == tenantID &&
			entry.ContractID == contractID &&
			entry.Period.Start.Equal(period.Start) &&
			entry.Period.End.Equal(period.End) {
			result = append(result, entry)
		}
	}

	return result, nil
}

func (s *workflowStore) ListActualRevenue(
	ctx context.Context,
	tenantID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) ([]revenuedomain.ActualRevenueEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := make([]revenuedomain.ActualRevenueEntry, 0, len(s.actual))
	for _, entry := range s.actual {
		if entry.TenantID == tenantID &&
			entry.ContractID == contractID &&
			entry.Period.Start.Equal(period.Start) &&
			entry.Period.End.Equal(period.End) {
			result = append(result, entry)
		}
	}

	return result, nil
}

func (s *workflowStore) CreateLeakageCase(
	ctx context.Context,
	c leakage.Case,
	evidence []leakage.Evidence,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.cases = append(s.cases, c)
	return nil
}

func issuedInvoice(
	invoiceID uuid.UUID,
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) billing.Invoice {
	return billing.Invoice{
		ID:          invoiceID,
		TenantID:    tenantID,
		CustomerID:  customerID,
		ContractID:  contractID,
		Period:      period,
		TotalAmount: valueobject.MustMoney("USD", 464_000),
		Status:      billing.InvoiceStatusIssued,
	}
}

func invoiceLine(
	invoiceID uuid.UUID,
	billableItemID uuid.UUID,
	lineTotal valueobject.Money,
) billing.InvoiceLine {
	return billing.InvoiceLine{
		ID:             uuid.New(),
		InvoiceID:      invoiceID,
		BillableItemID: billableItemID,
		LineTotal:      lineTotal,
	}
}

func usageRecord(
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	billableItemID uuid.UUID,
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
		Unit:           "events",
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
