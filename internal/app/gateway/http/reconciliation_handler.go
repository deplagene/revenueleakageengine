package http

import (
	"fmt"
	stdhttp "net/http"
	"strings"
	"time"

	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	revenueservice "github.com/deplagene/revenueleakageengine/internal/service/revenue"
	"github.com/google/uuid"
)

type runReconciliationRequest struct {
	TenantID                 string               `json:"tenant_id"`
	CustomerID               string               `json:"customer_id"`
	ContractID               string               `json:"contract_id"`
	BillableItemID           string               `json:"billable_item_id"`
	Currency                 string               `json:"currency"`
	TraceID                  string               `json:"trace_id"`
	MinimumLeakageMinorUnits int64                `json:"minimum_leakage_minor_units"`
	Period                   billingPeriodRequest `json:"period"`
	Pricing                  pricingRequest       `json:"pricing"`
	UsageRecords             []usageRecordRequest `json:"usage_records"`
	Invoices                 []invoiceRequest     `json:"invoices"`
	InvoiceLines             []invoiceLineRequest `json:"invoice_lines"`
}

type billingPeriodRequest struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type pricingRequest struct {
	BaseFeeMinorUnits          int64  `json:"base_fee_minor_units"`
	IncludedQuantity           int64  `json:"included_quantity"`
	OverageUnitPriceMinorUnits int64  `json:"overage_unit_price_minor_units"`
	Unit                       string `json:"unit"`
}

type usageRecordRequest struct {
	ID           string `json:"id"`
	ExternalID   string `json:"external_id"`
	UsageTime    string `json:"usage_time"`
	Quantity     int64  `json:"quantity"`
	SourceSystem string `json:"source_system"`
}

type invoiceRequest struct {
	ID                    string `json:"id"`
	ExternalID            string `json:"external_id"`
	Number                string `json:"number"`
	IssuedAt              string `json:"issued_at"`
	DueAt                 string `json:"due_at"`
	TotalAmountMinorUnits int64  `json:"total_amount_minor_units"`
	Status                string `json:"status"`
}

type invoiceLineRequest struct {
	ID                  string `json:"id"`
	InvoiceID           string `json:"invoice_id"`
	BillableItemID      string `json:"billable_item_id"`
	Description         string `json:"description"`
	Quantity            int64  `json:"quantity"`
	UnitPriceMinorUnits int64  `json:"unit_price_minor_units"`
	DiscountMinorUnits  int64  `json:"discount_amount_minor_units"`
	TaxMinorUnits       int64  `json:"tax_amount_minor_units"`
	LineTotalMinorUnits int64  `json:"line_total_minor_units"`
	SourceRef           string `json:"source_ref"`
}

type runReconciliationResponse struct {
	RunID                   string                  `json:"run_id"`
	ExpectedEntryID         string                  `json:"expected_entry_id"`
	ActualEntryCount        int                     `json:"actual_entry_count"`
	DiffCount               int                     `json:"diff_count"`
	CaseCount               int                     `json:"case_count"`
	LeakageAmountMinorUnits int64                   `json:"leakage_amount_minor_units"`
	Currency                string                  `json:"currency"`
	Cases                   []runReconciliationCase `json:"cases"`
}

type runReconciliationCase struct {
	ID                         string `json:"id"`
	Type                       string `json:"type"`
	Severity                   string `json:"severity"`
	Status                     string `json:"status"`
	ExpectedAmountMinorUnits   int64  `json:"expected_amount_minor_units"`
	ActualAmountMinorUnits     int64  `json:"actual_amount_minor_units"`
	LeakageAmountMinorUnits    int64  `json:"leakage_amount_minor_units"`
	ConfidenceScoreBasisPoints uint16 `json:"confidence_score_basis_points"`
}

func (h *Handler) handleRunReconciliation(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var request runReconciliationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	command, err := request.toCommand()
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	result, err := h.reconciliation.RunRevenueLeakageCheck(r.Context(), command)
	if err != nil {
		writeError(w, stdhttp.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, newRunReconciliationResponse(result))
}

func (r runReconciliationRequest) toCommand() (appreconciliation.RunRevenueLeakageCheckCommand, error) {
	tenantID, err := parseUUID("tenant id", r.TenantID)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	customerID, err := parseUUID("customer id", r.CustomerID)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	contractID, err := parseUUID("contract id", r.ContractID)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	billableItemID, err := parseUUID("billable item id", r.BillableItemID)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	period, err := r.Period.toValueObject()
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	pricing, err := r.Pricing.toModel(r.Currency)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	usageRecords, err := r.toUsageRecords(tenantID, customerID, contractID, billableItemID)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	invoices, err := r.toInvoices(tenantID, customerID, contractID, period)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	invoiceLines, err := r.toInvoiceLines()
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	minimumLeakageAmount, err := valueobject.NewMoney(r.Currency, r.MinimumLeakageMinorUnits)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, fmt.Errorf("build minimum leakage amount: %w", err)
	}

	return appreconciliation.RunRevenueLeakageCheckCommand{
		Expected: revenueservice.CalculateExpectedRevenueCommand{
			TenantID:       tenantID,
			CustomerID:     customerID,
			ContractID:     contractID,
			BillableItemID: billableItemID,
			Period:         period,
			Pricing:        pricing,
			UsageRecords:   usageRecords,
			TraceID:        r.TraceID,
		},
		Actual: revenueservice.BuildActualRevenueCommand{
			TenantID:     tenantID,
			CustomerID:   customerID,
			ContractID:   contractID,
			Period:       period,
			Invoices:     invoices,
			InvoiceLines: invoiceLines,
			TraceID:      r.TraceID,
		},
		TraceID:              r.TraceID,
		Currency:             r.Currency,
		MinimumLeakageAmount: minimumLeakageAmount,
	}, nil
}

func (r billingPeriodRequest) toValueObject() (valueobject.BillingPeriod, error) {
	start, err := parseTime("period start", r.Start)
	if err != nil {
		return valueobject.BillingPeriod{}, err
	}

	end, err := parseTime("period end", r.End)
	if err != nil {
		return valueobject.BillingPeriod{}, err
	}

	period, err := valueobject.NewBillingPeriod(start, end)
	if err != nil {
		return valueobject.BillingPeriod{}, fmt.Errorf("build billing period: %w", err)
	}

	return period, nil
}

func (r pricingRequest) toModel(currency string) (revenueservice.FixedUsagePricing, error) {
	baseFee, err := valueobject.NewMoney(currency, r.BaseFeeMinorUnits)
	if err != nil {
		return revenueservice.FixedUsagePricing{}, fmt.Errorf("build base fee: %w", err)
	}

	overageUnitPrice, err := valueobject.NewMoney(currency, r.OverageUnitPriceMinorUnits)
	if err != nil {
		return revenueservice.FixedUsagePricing{}, fmt.Errorf("build overage unit price: %w", err)
	}

	return revenueservice.FixedUsagePricing{
		BaseFee:          baseFee,
		IncludedQuantity: r.IncludedQuantity,
		OverageUnitPrice: overageUnitPrice,
		Unit:             r.Unit,
	}, nil
}

func (r runReconciliationRequest) toUsageRecords(
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	billableItemID uuid.UUID,
) ([]billing.UsageRecord, error) {
	records := make([]billing.UsageRecord, 0, len(r.UsageRecords))
	for _, item := range r.UsageRecords {
		record, err := item.toDomain(
			tenantID,
			customerID,
			contractID,
			billableItemID,
			r.Pricing.Unit,
			r.TraceID,
		)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	return records, nil
}

func (r usageRecordRequest) toDomain(
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	billableItemID uuid.UUID,
	unit string,
	traceID string,
) (billing.UsageRecord, error) {
	id, err := parseUUID("usage record id", r.ID)
	if err != nil {
		return billing.UsageRecord{}, err
	}

	usageTime, err := parseTime("usage time", r.UsageTime)
	if err != nil {
		return billing.UsageRecord{}, err
	}

	return billing.UsageRecord{
		ID:             id,
		TenantID:       tenantID,
		CustomerID:     customerID,
		ContractID:     contractID,
		BillableItemID: billableItemID,
		ExternalID:     r.ExternalID,
		UsageTime:      usageTime,
		Quantity:       r.Quantity,
		Unit:           unit,
		SourceSystem:   r.SourceSystem,
		TraceID:        traceID,
		Metadata:       map[string]any{},
	}, nil
}

func (r runReconciliationRequest) toInvoices(
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
) ([]billing.Invoice, error) {
	invoices := make([]billing.Invoice, 0, len(r.Invoices))
	for _, item := range r.Invoices {
		invoice, err := item.toDomain(
			tenantID,
			customerID,
			contractID,
			period,
			r.Currency,
		)
		if err != nil {
			return nil, err
		}

		invoices = append(invoices, invoice)
	}

	return invoices, nil
}

func (r invoiceRequest) toDomain(
	tenantID uuid.UUID,
	customerID uuid.UUID,
	contractID uuid.UUID,
	period valueobject.BillingPeriod,
	currency string,
) (billing.Invoice, error) {
	id, err := parseUUID("invoice id", r.ID)
	if err != nil {
		return billing.Invoice{}, err
	}

	issuedAt, err := parseTime("invoice issued at", r.IssuedAt)
	if err != nil {
		return billing.Invoice{}, err
	}

	dueAt, err := parseTime("invoice due at", r.DueAt)
	if err != nil {
		return billing.Invoice{}, err
	}

	totalAmount, err := valueobject.NewMoney(currency, r.TotalAmountMinorUnits)
	if err != nil {
		return billing.Invoice{}, fmt.Errorf("build invoice total amount: %w", err)
	}

	status, err := parseInvoiceStatus(r.Status)
	if err != nil {
		return billing.Invoice{}, err
	}

	return billing.Invoice{
		ID:          id,
		TenantID:    tenantID,
		CustomerID:  customerID,
		ContractID:  contractID,
		ExternalID:  r.ExternalID,
		Number:      r.Number,
		Period:      period,
		IssuedAt:    issuedAt,
		DueAt:       dueAt,
		TotalAmount: totalAmount,
		Status:      status,
	}, nil
}

func (r runReconciliationRequest) toInvoiceLines() ([]billing.InvoiceLine, error) {
	lines := make([]billing.InvoiceLine, 0, len(r.InvoiceLines))
	for _, item := range r.InvoiceLines {
		line, err := item.toDomain(r.Currency)
		if err != nil {
			return nil, err
		}

		lines = append(lines, line)
	}

	return lines, nil
}

func (r invoiceLineRequest) toDomain(currency string) (billing.InvoiceLine, error) {
	id, err := parseUUID("invoice line id", r.ID)
	if err != nil {
		return billing.InvoiceLine{}, err
	}

	invoiceID, err := parseUUID("invoice line invoice id", r.InvoiceID)
	if err != nil {
		return billing.InvoiceLine{}, err
	}

	billableItemID, err := parseUUID("invoice line billable item id", r.BillableItemID)
	if err != nil {
		return billing.InvoiceLine{}, err
	}

	unitPrice, err := valueobject.NewMoney(currency, r.UnitPriceMinorUnits)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build invoice line unit price: %w", err)
	}

	discountAmount, err := valueobject.NewMoney(currency, r.DiscountMinorUnits)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build invoice line discount amount: %w", err)
	}

	taxAmount, err := valueobject.NewMoney(currency, r.TaxMinorUnits)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build invoice line tax amount: %w", err)
	}

	lineTotal, err := valueobject.NewMoney(currency, r.LineTotalMinorUnits)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build invoice line total amount: %w", err)
	}

	return billing.InvoiceLine{
		ID:              id,
		InvoiceID:       invoiceID,
		BillableItemID:  billableItemID,
		Description:     r.Description,
		Quantity:        r.Quantity,
		UnitPrice:       unitPrice,
		DiscountAmount:  discountAmount,
		TaxAmount:       taxAmount,
		LineTotal:       lineTotal,
		SourceRef:       r.SourceRef,
		PricingSnapshot: map[string]any{},
	}, nil
}

func newRunReconciliationResponse(
	result appreconciliation.RunRevenueLeakageCheckResult,
) runReconciliationResponse {
	cases := make([]runReconciliationCase, 0, len(result.ReconciliationResult.Cases))
	for _, item := range result.ReconciliationResult.Cases {
		cases = append(cases, runReconciliationCase{
			ID:                         item.ID.String(),
			Type:                       string(item.Type),
			Severity:                   string(item.Severity),
			Status:                     string(item.Status),
			ExpectedAmountMinorUnits:   item.ExpectedAmount.MinorUnits,
			ActualAmountMinorUnits:     item.ActualAmount.MinorUnits,
			LeakageAmountMinorUnits:    item.LeakageAmount.MinorUnits,
			ConfidenceScoreBasisPoints: item.ConfidenceScore.BasisPoints,
		})
	}

	return runReconciliationResponse{
		RunID:                   result.ReconciliationResult.RunID.String(),
		ExpectedEntryID:         result.ExpectedEntry.String(),
		ActualEntryCount:        result.ActualEntryCount,
		DiffCount:               result.ReconciliationResult.DiffCount,
		CaseCount:               result.ReconciliationResult.CaseCount,
		LeakageAmountMinorUnits: result.ReconciliationResult.LeakageAmount.MinorUnits,
		Currency:                result.ReconciliationResult.LeakageAmount.Currency,
		Cases:                   cases,
	}
}

func parseUUID(field, raw string) (uuid.UUID, error) {
	value, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse %s: %w", field, err)
	}

	return value, nil
}

func parseTime(field, raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", field, err)
	}

	return value.UTC(), nil
}

func parseInvoiceStatus(raw string) (billing.InvoiceStatus, error) {
	switch billing.InvoiceStatus(strings.TrimSpace(raw)) {
	case billing.InvoiceStatusDraft:
		return billing.InvoiceStatusDraft, nil
	case billing.InvoiceStatusIssued:
		return billing.InvoiceStatusIssued, nil
	case billing.InvoiceStatusPaid:
		return billing.InvoiceStatusPaid, nil
	case billing.InvoiceStatusVoid:
		return billing.InvoiceStatusVoid, nil
	case billing.InvoiceStatusOverdue:
		return billing.InvoiceStatusOverdue, nil
	default:
		return "", fmt.Errorf("parse invoice status: invalid status %q", raw)
	}
}
