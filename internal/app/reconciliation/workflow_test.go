package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	ingestionapp "github.com/deplagene/revenueleakageengine/internal/app/ingestion"
	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	contractdomain "github.com/deplagene/revenueleakageengine/internal/domain/contract"
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

	tenantID := uuid.New()
	customerID := uuid.New()
	contractID := uuid.New()
	billableItemID := uuid.New()
	invoiceID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")

	contractQueries := &workflowContractQueries{
		contractDoc: &contractdomain.Contract{
			ID:         contractID,
			TenantID:   tenantID,
			CustomerID: customerID,
			Currency:   "USD",
		},
		terms: []*contractdomain.Term{
			pricingTerm(contractdomain.TermTypeFixedFee, map[string]any{
				"code":               "platform_subscription",
				"amount_minor_units": float64(500_000),
				"currency":           "USD",
			}),
			pricingTerm(contractdomain.TermTypeUsageRate, map[string]any{
				"code":                   "platform_subscription",
				"unit_price_minor_units": float64(1),
				"included_quantity":      float64(100_000),
				"unit":                   "events",
				"currency":               "USD",
			}),
		},
		billableItem: &contractdomain.BillableItem{
			ID:       billableItemID,
			TenantID: tenantID,
			Code:     "platform_subscription",
			Unit:     "events",
		},
	}
	ingestionQueries := &workflowIngestionQueries{
		usageRecords: []billing.UsageRecord{
			usageRecord(tenantID, customerID, contractID, billableItemID, 180_000, period.Start),
		},
		invoices: []billing.Invoice{
			issuedInvoice(invoiceID, tenantID, customerID, contractID, period),
		},
		invoiceLines: []billing.InvoiceLine{
			invoiceLine(invoiceID, billableItemID, valueobject.MustMoney("USD", 464_000)),
		},
	}

	workflow, err := NewRevenueLeakageWorkflow(
		revenueService,
		reconciliationService,
		contractQueries,
		ingestionQueries,
	)
	if err != nil {
		t.Fatalf("NewRevenueLeakageWorkflow() error = %v", err)
	}

	result, err := workflow.RunRevenueLeakageCheck(ctx, RunRevenueLeakageCheckCommand{
		TenantID:    tenantID,
		ContractID:  contractID,
		PeriodStart: period.Start,
		PeriodEnd:   period.End,
		TraceID:     "business-trace-1",
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

	if store.cases[0].ReconciliationRunID != result.ReconciliationResult.RunID {
		t.Fatalf(
			"case reconciliation run id = %s, want %s",
			store.cases[0].ReconciliationRunID,
			result.ReconciliationResult.RunID,
		)
	}

	if len(store.completedRuns) != 1 {
		t.Fatalf("completed run count = %d, want 1", len(store.completedRuns))
	}

	if store.completedRuns[0].CaseCount != 1 {
		t.Fatalf("completed run case count = %d, want 1", store.completedRuns[0].CaseCount)
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

	tenantID := uuid.New()
	contractID := uuid.New()
	period := mustPeriod(t, "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z")
	workflow, err := NewRevenueLeakageWorkflow(
		revenueservice.NewService(revenueservice.WithStore(store)),
		reconciliationService,
		&workflowContractQueries{
			contractDoc: &contractdomain.Contract{
				ID:         contractID,
				TenantID:   uuid.New(),
				CustomerID: uuid.New(),
				Currency:   "USD",
			},
		},
		&workflowIngestionQueries{},
	)
	if err != nil {
		t.Fatalf("NewRevenueLeakageWorkflow() error = %v", err)
	}

	_, err = workflow.RunRevenueLeakageCheck(context.Background(), RunRevenueLeakageCheckCommand{
		TenantID:    tenantID,
		ContractID:  contractID,
		PeriodStart: period.Start,
		PeriodEnd:   period.End,
	})
	if !errors.Is(err, ErrWorkflowScopeMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrWorkflowScopeMismatch)
	}
}

type workflowContractQueries struct {
	contractDoc  *contractdomain.Contract
	terms        []*contractdomain.Term
	billableItem *contractdomain.BillableItem
}

func (q *workflowContractQueries) GetContract(
	ctx context.Context,
	id uuid.UUID,
) (*contractdomain.Contract, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if q.contractDoc == nil || q.contractDoc.ID != id {
		return nil, fmt.Errorf("contract %s not found", id)
	}

	return q.contractDoc, nil
}

func (q *workflowContractQueries) GetEffectiveTerms(
	ctx context.Context,
	contractID uuid.UUID,
	at time.Time,
) ([]*contractdomain.Term, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return q.terms, nil
}

func (q *workflowContractQueries) GetBillableItemByCode(
	ctx context.Context,
	tenantID uuid.UUID,
	code string,
) (*contractdomain.BillableItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if q.billableItem == nil || q.billableItem.TenantID != tenantID || q.billableItem.Code != code {
		return nil, fmt.Errorf("billable item %q not found", code)
	}

	return q.billableItem, nil
}

type workflowIngestionQueries struct {
	usageRecords []billing.UsageRecord
	invoices     []billing.Invoice
	invoiceLines []billing.InvoiceLine
}

func (q *workflowIngestionQueries) ListUsageRecordsForContractPeriod(
	ctx context.Context,
	query ingestionapp.ContractPeriodQuery,
) ([]billing.UsageRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return q.usageRecords, nil
}

func (q *workflowIngestionQueries) ListInvoicesForContractPeriod(
	ctx context.Context,
	query ingestionapp.ContractPeriodQuery,
) ([]billing.Invoice, []billing.InvoiceLine, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	return q.invoices, q.invoiceLines, nil
}

type workflowStore struct {
	expected      []revenuedomain.ExpectedRevenueEntry
	actual        []revenuedomain.ActualRevenueEntry
	cases         []leakage.Case
	runs          []reconciliationservice.ReconciliationRun
	completedRuns []reconciliationservice.ReconciliationRun
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

func (s *workflowStore) ListReconciliationRuns(
	ctx context.Context,
	cmd reconciliationservice.ListReconciliationRunsCommand,
) ([]reconciliationservice.ReconciliationRunSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *workflowStore) GetReconciliationRun(
	ctx context.Context,
	cmd reconciliationservice.GetReconciliationRunCommand,
) (reconciliationservice.ReconciliationRunSummary, error) {
	if err := ctx.Err(); err != nil {
		return reconciliationservice.ReconciliationRunSummary{}, err
	}

	return reconciliationservice.ReconciliationRunSummary{}, nil
}

func (s *workflowStore) CreateReconciliationRun(
	ctx context.Context,
	run reconciliationservice.ReconciliationRun,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.runs = append(s.runs, run)
	return nil
}

func (s *workflowStore) CompleteReconciliationRun(
	ctx context.Context,
	run reconciliationservice.ReconciliationRun,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.completedRuns = append(s.completedRuns, run)
	return nil
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

func pricingTerm(
	termType contractdomain.TermType,
	expression map[string]any,
) *contractdomain.Term {
	return &contractdomain.Term{
		ID:         uuid.New(),
		Type:       termType,
		Expression: expression,
	}
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
	usageTime time.Time,
) billing.UsageRecord {
	return billing.UsageRecord{
		ID:             uuid.New(),
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		UsageTime:      usageTime,
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
