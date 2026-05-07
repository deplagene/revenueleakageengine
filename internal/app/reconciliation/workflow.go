// Package reconciliation contains application-level use cases that orchestrate
// revenue calculation and reconciliation services.
package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	ingestionapp "github.com/deplagene/revenueleakageengine/internal/app/ingestion"
	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	revenueservice "github.com/deplagene/revenueleakageengine/internal/service/revenue"
	"github.com/google/uuid"
)

var (
	// ErrRevenueServiceRequired reports that the workflow has no revenue
	// service dependency.
	ErrRevenueServiceRequired = errors.New("revenue service is required")
	// ErrReconciliationServiceRequired reports that the workflow has no
	// reconciliation service dependency.
	ErrReconciliationServiceRequired = errors.New("reconciliation service is required")
	// ErrContractQueriesRequired reports that the workflow cannot load
	// contract pricing data.
	ErrContractQueriesRequired = errors.New("contract queries are required")
	// ErrIngestionQueriesRequired reports that the workflow cannot load usage
	// and invoice facts.
	ErrIngestionQueriesRequired = errors.New("ingestion queries are required")
	// ErrContractRequired reports that the workflow loaded no contract record.
	ErrContractRequired = errors.New("contract is required")
	// ErrWorkflowScopeMismatch reports that requested and loaded business
	// scopes do not match.
	ErrWorkflowScopeMismatch = errors.New("workflow scope mismatch")
	// ErrPricingTermsRequired reports that a contract has no effective pricing
	// term the MVP calculator can use.
	ErrPricingTermsRequired = errors.New("pricing terms are required")
	// ErrPricingExpressionInvalid reports a malformed contract term expression.
	ErrPricingExpressionInvalid = errors.New("pricing expression is invalid")
)

// RevenueLeakageWorkflow coordinates the MVP flow:
// expected revenue -> actual revenue -> reconciliation.
type RevenueLeakageWorkflow struct {
	revenue          *revenueservice.Service
	reconciliation   *reconciliationservice.Service
	contractQueries  ContractQueries
	ingestionQueries IngestionQueries
	events           eventSink
	now              func() time.Time
}

type eventSink interface {
	Append(ctx context.Context, envelope appevent.Envelope) error
	AppendBatch(ctx context.Context, envelopes []appevent.Envelope) error
}

type WorkflowOption func(*RevenueLeakageWorkflow)

func WithEventSink(events eventSink) WorkflowOption {
	return func(w *RevenueLeakageWorkflow) {
		w.events = events
	}
}

func WithClock(now func() time.Time) WorkflowOption {
	return func(w *RevenueLeakageWorkflow) {
		if now != nil {
			w.now = now
		}
	}
}

type ContractQueries interface {
	GetContract(ctx context.Context, id uuid.UUID) (*contract.Contract, error)
	GetEffectiveTerms(ctx context.Context, contractID uuid.UUID, at time.Time) ([]*contract.Term, error)
	GetBillableItemByCode(ctx context.Context, tenantID uuid.UUID, code string) (*contract.BillableItem, error)
}

type IngestionQueries interface {
	ListUsageRecordsForContractPeriod(
		ctx context.Context,
		query ingestionapp.ContractPeriodQuery,
	) ([]billing.UsageRecord, error)
	ListInvoicesForContractPeriod(
		ctx context.Context,
		query ingestionapp.ContractPeriodQuery,
	) ([]billing.Invoice, []billing.InvoiceLine, error)
}

// NewRevenueLeakageWorkflow creates an application workflow from business
// services. It does not own storage or transport dependencies directly.
func NewRevenueLeakageWorkflow(
	revenue *revenueservice.Service,
	reconciliation *reconciliationservice.Service,
	contractQueries ContractQueries,
	ingestionQueries IngestionQueries,
	opts ...WorkflowOption,
) (*RevenueLeakageWorkflow, error) {
	if revenue == nil {
		return nil, ErrRevenueServiceRequired
	}

	if reconciliation == nil {
		return nil, ErrReconciliationServiceRequired
	}

	if contractQueries == nil {
		return nil, ErrContractQueriesRequired
	}

	if ingestionQueries == nil {
		return nil, ErrIngestionQueriesRequired
	}

	workflow := &RevenueLeakageWorkflow{
		revenue:          revenue,
		reconciliation:   reconciliation,
		contractQueries:  contractQueries,
		ingestionQueries: ingestionQueries,
		now:              time.Now,
	}

	for _, opt := range opts {
		opt(workflow)
	}

	return workflow, nil
}

// RunRevenueLeakageCheckCommand contains the inputs required for one full MVP
// leakage check.
type RunRevenueLeakageCheckCommand struct {
	TenantID             uuid.UUID
	ContractID           uuid.UUID
	PeriodStart          time.Time
	PeriodEnd            time.Time
	RunID                uuid.UUID
	TraceID              string
	Currency             string
	MinimumLeakageAmount valueobject.Money
}

// Validate checks that the workflow has enough scope to load DB-backed facts.
func (c RunRevenueLeakageCheckCommand) Validate() error {
	if c.TenantID == uuid.Nil {
		return billing.ErrTenantRequired
	}

	if c.ContractID == uuid.Nil {
		return billing.ErrContractRequired
	}

	if _, err := c.BillingPeriod(); err != nil {
		return err
	}

	if c.MinimumLeakageAmount.Currency != "" {
		return c.MinimumLeakageAmount.Validate()
	}

	return nil
}

// BillingPeriod builds the normalized period used across revenue and
// reconciliation services.
func (c RunRevenueLeakageCheckCommand) BillingPeriod() (valueobject.BillingPeriod, error) {
	return valueobject.NewBillingPeriod(c.PeriodStart, c.PeriodEnd)
}

// RunRevenueLeakageCheckResult contains the generated ledger counts and
// reconciliation outcome.
type RunRevenueLeakageCheckResult struct {
	ExpectedEntry        uuid.UUID
	ActualEntryCount     int
	ReconciliationResult reconciliationservice.ReconcilePeriodResult
}

// RunRevenueLeakageCheck calculates expected revenue, builds actual revenue,
// persists both ledgers, and then reconciles them.
func (w *RevenueLeakageWorkflow) RunRevenueLeakageCheck(
	ctx context.Context,
	cmd RunRevenueLeakageCheckCommand,
) (RunRevenueLeakageCheckResult, error) {
	const op = "app.reconciliation.RunRevenueLeakageCheck"

	if err := ctx.Err(); err != nil {
		return RunRevenueLeakageCheckResult{}, err
	}

	if err := cmd.Validate(); err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: %w", op, err)
	}

	period, err := cmd.BillingPeriod()
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: build billing period: %w", op, err)
	}

	contractDoc, err := w.contractQueries.GetContract(ctx, cmd.ContractID)
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: get contract: %w", op, err)
	}

	if contractDoc == nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: %w", op, ErrContractRequired)
	}

	if contractDoc.TenantID != cmd.TenantID {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: %w", op, ErrWorkflowScopeMismatch)
	}

	terms, err := w.contractQueries.GetEffectiveTerms(ctx, cmd.ContractID, period.End)
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: get effective terms: %w", op, err)
	}

	pricingDefinition, err := pricingDefinitionFromTerms(terms, contractDoc.Currency)
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: build pricing from terms: %w", op, err)
	}

	billableItem, err := w.contractQueries.GetBillableItemByCode(
		ctx,
		cmd.TenantID,
		pricingDefinition.itemCode,
	)
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: get billable item: %w", op, err)
	}

	if billableItem == nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: %w", op, contract.ErrBillableItemIDRequired)
	}

	if pricingDefinition.unit == "" {
		pricingDefinition.unit = billableItem.Unit
	}

	pricing, err := pricingDefinition.toFixedUsagePricing()
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: build pricing model: %w", op, err)
	}

	ingestionQuery := ingestionapp.ContractPeriodQuery{
		TenantID:   cmd.TenantID,
		ContractID: cmd.ContractID,
		Period:     period,
	}

	usageRecords, err := w.ingestionQueries.ListUsageRecordsForContractPeriod(ctx, ingestionQuery)
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: list usage records: %w", op, err)
	}

	invoices, invoiceLines, err := w.ingestionQueries.ListInvoicesForContractPeriod(ctx, ingestionQuery)
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: list invoices: %w", op, err)
	}

	traceID := traceIDOrNew(cmd.TraceID)
	expected, err := w.revenue.CalculateAndSaveExpectedRevenue(ctx, revenueservice.CalculateExpectedRevenueCommand{
		TenantID:       cmd.TenantID,
		CustomerID:     contractDoc.CustomerID,
		ContractID:     cmd.ContractID,
		BillableItemID: billableItem.ID,
		Period:         period,
		Pricing:        pricing,
		UsageRecords:   usageRecords,
		Version:        1,
		TraceID:        traceID,
	})
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: calculate expected revenue: %w", op, err)
	}

	actualEntries, err := w.revenue.BuildAndSaveActualRevenue(ctx, revenueservice.BuildActualRevenueCommand{
		TenantID:     cmd.TenantID,
		CustomerID:   contractDoc.CustomerID,
		ContractID:   cmd.ContractID,
		Period:       period,
		Invoices:     invoices,
		InvoiceLines: invoiceLines,
		TraceID:      traceID,
	})
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: build actual revenue: %w", op, err)
	}

	reconciliationResult, err := w.reconciliation.ReconcilePeriod(ctx, reconciliationservice.ReconcilePeriodCommand{
		TenantID:             cmd.TenantID,
		ContractID:           cmd.ContractID,
		Period:               period,
		RunID:                runIDOrNew(cmd.RunID),
		TraceID:              traceID,
		Currency:             currencyOrDefault(cmd.Currency, expected.ExpectedAmount.Currency),
		MinimumLeakageAmount: minimumLeakageOrZero(cmd.MinimumLeakageAmount, expected.ExpectedAmount.Currency),
	})
	if err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: reconcile period: %w", op, err)
	}

	result := RunRevenueLeakageCheckResult{
		ExpectedEntry:        expected.ID,
		ActualEntryCount:     len(actualEntries),
		ReconciliationResult: reconciliationResult,
	}

	if err := w.publishReconciliationCompleted(ctx, cmd, result, traceID); err != nil {
		return RunRevenueLeakageCheckResult{}, fmt.Errorf("%s: publish reconciliation events: %w", op, err)
	}

	return result, nil
}

type fixedUsagePricingDefinition struct {
	itemCode                  string
	currency                  string
	baseFeeMinorUnits         int64
	includedQuantity          int64
	overageUnitPriceMinorUnit int64
	unit                      string
}

func (d fixedUsagePricingDefinition) toFixedUsagePricing() (revenueservice.FixedUsagePricing, error) {
	baseFee, err := valueobject.NewMoney(d.currency, d.baseFeeMinorUnits)
	if err != nil {
		return revenueservice.FixedUsagePricing{}, fmt.Errorf("build base fee: %w", err)
	}

	overageUnitPrice, err := valueobject.NewMoney(d.currency, d.overageUnitPriceMinorUnit)
	if err != nil {
		return revenueservice.FixedUsagePricing{}, fmt.Errorf("build overage unit price: %w", err)
	}

	pricing := revenueservice.FixedUsagePricing{
		BaseFee:          baseFee,
		IncludedQuantity: d.includedQuantity,
		OverageUnitPrice: overageUnitPrice,
		Unit:             d.unit,
	}
	if err := pricing.Validate(); err != nil {
		return revenueservice.FixedUsagePricing{}, err
	}

	return pricing, nil
}

func pricingDefinitionFromTerms(
	terms []*contract.Term,
	contractCurrency string,
) (fixedUsagePricingDefinition, error) {
	definition := fixedUsagePricingDefinition{
		currency: contractCurrency,
	}
	foundPricing := false

	for _, term := range terms {
		if term == nil {
			continue
		}

		switch {
		case isFixedFeeTerm(term.Type):
			foundPricing = true
			baseFee, err := requiredInt64Expression(term.Expression, "amount_minor_units", "amount")
			if err != nil {
				return fixedUsagePricingDefinition{}, fmt.Errorf("fixed fee term %s: %w", term.ID, err)
			}

			definition.baseFeeMinorUnits = baseFee
			definition.applySharedExpression(term.Expression)
		case isUsageRateTerm(term.Type):
			foundPricing = true
			unitPrice, err := requiredInt64Expression(
				term.Expression,
				"unit_price_minor_units",
				"overage_unit_price_minor_units",
				"amount_minor_units",
				"amount",
			)
			if err != nil {
				return fixedUsagePricingDefinition{}, fmt.Errorf("usage rate term %s: %w", term.ID, err)
			}

			definition.overageUnitPriceMinorUnit = unitPrice
			if includedQuantity, ok, err := optionalInt64Expression(
				term.Expression,
				"included_quantity",
			); err != nil {
				return fixedUsagePricingDefinition{}, fmt.Errorf("usage rate term %s: %w", term.ID, err)
			} else if ok {
				definition.includedQuantity = includedQuantity
			}

			definition.applySharedExpression(term.Expression)
		}
	}

	if !foundPricing {
		return fixedUsagePricingDefinition{}, ErrPricingTermsRequired
	}

	if strings.TrimSpace(definition.itemCode) == "" {
		return fixedUsagePricingDefinition{}, contract.ErrBillableItemCodeRequired
	}

	return definition, nil
}

func (d *fixedUsagePricingDefinition) applySharedExpression(expression map[string]any) {
	if itemCode, ok := optionalStringExpression(expression, "code", "billable_item_code"); ok {
		if d.itemCode == "" {
			d.itemCode = itemCode
		}
	}

	if currency, ok := optionalStringExpression(expression, "currency"); ok {
		d.currency = currency
	}

	if unit, ok := optionalStringExpression(expression, "unit"); ok {
		d.unit = unit
	}
}

func isFixedFeeTerm(termType contract.TermType) bool {
	return termType == contract.TermTypeFixedFee || termType == contract.TermTypeBasePrice
}

func isUsageRateTerm(termType contract.TermType) bool {
	return termType == contract.TermTypeUsageRate ||
		termType == contract.TermTypeTieredPrice ||
		termType == contract.TermType("usage_price")
}

func requiredInt64Expression(expression map[string]any, keys ...string) (int64, error) {
	value, ok, err := optionalInt64Expression(expression, keys...)
	if err != nil {
		return 0, err
	}

	if !ok {
		return 0, fmt.Errorf("%w: missing %s", ErrPricingExpressionInvalid, strings.Join(keys, " or "))
	}

	return value, nil
}

func optionalInt64Expression(expression map[string]any, keys ...string) (int64, bool, error) {
	for _, key := range keys {
		raw, ok := expression[key]
		if !ok {
			continue
		}

		value, err := int64ExpressionValue(key, raw)
		if err != nil {
			return 0, false, err
		}

		return value, true, nil
	}

	return 0, false, nil
}

func int64ExpressionValue(key string, raw any) (int64, error) {
	switch value := raw.(type) {
	case int:
		return int64(value), nil
	case int8:
		return int64(value), nil
	case int16:
		return int64(value), nil
	case int32:
		return int64(value), nil
	case int64:
		return value, nil
	case uint:
		return uint64ExpressionValue(key, uint64(value))
	case uint8:
		return int64(value), nil
	case uint16:
		return int64(value), nil
	case uint32:
		return int64(value), nil
	case uint64:
		return uint64ExpressionValue(key, value)
	case float32:
		return int64FloatExpressionValue(key, float64(value))
	case float64:
		return int64FloatExpressionValue(key, value)
	case json.Number:
		return int64JSONNumberExpressionValue(key, value)
	case string:
		return int64StringExpressionValue(key, value)
	default:
		return 0, fmt.Errorf("%w: %s must be an integer", ErrPricingExpressionInvalid, key)
	}
}

func uint64ExpressionValue(key string, value uint64) (int64, error) {
	if value > uint64(^uint64(0)>>1) {
		return 0, fmt.Errorf("%w: %s overflows int64", ErrPricingExpressionInvalid, key)
	}

	return int64(value), nil
}

func int64FloatExpressionValue(key string, value float64) (int64, error) {
	converted := int64(value)
	if value != float64(converted) {
		return 0, fmt.Errorf("%w: %s must be whole minor units", ErrPricingExpressionInvalid, key)
	}

	return converted, nil
}

func int64JSONNumberExpressionValue(key string, value json.Number) (int64, error) {
	parsed, err := value.Int64()
	if err == nil {
		return parsed, nil
	}

	floatValue, floatErr := value.Float64()
	if floatErr != nil {
		return 0, fmt.Errorf("%w: parse %s: %w", ErrPricingExpressionInvalid, key, errors.Join(err, floatErr))
	}

	return int64FloatExpressionValue(key, floatValue)
}

func int64StringExpressionValue(key, value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: parse %s: %w", ErrPricingExpressionInvalid, key, err)
	}

	return parsed, nil
}

func optionalStringExpression(expression map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		raw, ok := expression[key]
		if !ok {
			continue
		}

		value, ok := raw.(string)
		if !ok {
			continue
		}

		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		return value, true
	}

	return "", false
}

func traceIDOrNew(traceID string) string {
	traceID = strings.TrimSpace(traceID)
	if traceID != "" {
		return traceID
	}

	return uuid.New().String()
}

func runIDOrNew(runID uuid.UUID) uuid.UUID {
	if runID != uuid.Nil {
		return runID
	}

	return uuid.New()
}

func currencyOrDefault(currency, fallback string) string {
	currency = strings.TrimSpace(currency)
	if currency != "" {
		return currency
	}

	return fallback
}

func minimumLeakageOrZero(amount valueobject.Money, currency string) valueobject.Money {
	if amount.Currency != "" {
		return amount
	}

	return valueobject.ZeroMoney(currency)
}
