package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	ingestionapp "github.com/deplagene/revenueleakageengine/internal/app/ingestion"
	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
)

type ingestionCommands interface {
	IngestUsageRecords(ctx context.Context, records []billing.UsageRecord) error
	IngestInvoice(ctx context.Context, invoice billing.Invoice, lines []billing.InvoiceLine) error
}

type ingestionResponse struct {
	Status       string `json:"status"`
	UsageRecords int    `json:"usage_records,omitempty"`
	Invoices     int    `json:"invoices,omitempty"`
	InvoiceLines int    `json:"invoice_lines,omitempty"`
}

var ingestionValidationErrors = []error{
	ingestionapp.ErrUsageRecordsRequired,
	ingestionapp.ErrInvoiceRequired,
	ingestionapp.ErrInvoiceLinesRequired,
	billing.ErrUsageRecordIDRequired,
	billing.ErrTenantRequired,
	billing.ErrCustomerRequired,
	billing.ErrContractRequired,
	billing.ErrBillableItemRequired,
	billing.ErrExternalIDRequired,
	billing.ErrSourceSystemRequired,
	billing.ErrUsageTimeRequired,
	billing.ErrUsageQuantityInvalid,
	billing.ErrUsageUnitRequired,
	billing.ErrInvoiceIDRequired,
	billing.ErrInvoiceNumberRequired,
	billing.ErrInvoiceIssuedAtRequired,
	billing.ErrInvoiceDueAtInvalid,
	billing.ErrInvoiceStatusRequired,
	billing.ErrInvoiceStatusInvalid,
	billing.ErrInvoiceLineIDRequired,
	billing.ErrInvoiceLineInvoiceIDRequired,
	billing.ErrInvoiceLineDescriptionRequired,
	billing.ErrInvoiceLineQuantityInvalid,
	billing.ErrInvoiceLineCurrencyMismatch,
	valueobject.ErrInvalidBillingPeriod,
	valueobject.ErrCurrencyRequired,
}

func (h *Handler) handleIngestUsage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Records []struct {
			ID             string         `json:"id"`
			TenantID       string         `json:"tenant_id"`
			CustomerID     string         `json:"customer_id"`
			ContractID     string         `json:"contract_id"`
			BillableItemID string         `json:"billable_item_id"`
			ExternalID     string         `json:"external_id"`
			UsageTime      time.Time      `json:"usage_time"`
			Quantity       int64          `json:"quantity"`
			Unit           string         `json:"unit"`
			SourceSystem   string         `json:"source_system"`
			TraceID        string         `json:"trace_id"`
			Metadata       map[string]any `json:"metadata"`
		} `json:"records"`
	}

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	records := make([]billing.UsageRecord, 0, len(req.Records))
	for _, rec := range req.Records {
		id, err := optionalUUID("usage record id", rec.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		tenantID, err := parseUUID("usage record tenant id", rec.TenantID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		customerID, err := parseUUID("usage record customer id", rec.CustomerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		contractID, err := parseUUID("usage record contract id", rec.ContractID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		billableItemID, err := parseUUID("usage record billable item id", rec.BillableItemID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		records = append(records, billing.UsageRecord{
			ID:             id,
			TenantID:       tenantID,
			CustomerID:     customerID,
			ContractID:     contractID,
			BillableItemID: billableItemID,
			ExternalID:     rec.ExternalID,
			UsageTime:      rec.UsageTime,
			Quantity:       rec.Quantity,
			Unit:           rec.Unit,
			SourceSystem:   rec.SourceSystem,
			TraceID:        rec.TraceID,
			Metadata:       rec.Metadata,
		})
	}

	if err := h.ingestionCommands.IngestUsageRecords(r.Context(), records); err != nil {
		writeIngestionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ingestionResponse{
		Status:       "ok",
		UsageRecords: len(records),
	})
}

func (h *Handler) handleIngestInvoices(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID           string    `json:"id"`
		TenantID     string    `json:"tenant_id"`
		CustomerID   string    `json:"customer_id"`
		ContractID   string    `json:"contract_id"`
		ExternalID   string    `json:"external_id"`
		Number       string    `json:"number"`
		PeriodStart  time.Time `json:"period_start"`
		PeriodEnd    time.Time `json:"period_end"`
		IssuedAt     time.Time `json:"issued_at"`
		DueAt        time.Time `json:"due_at"`
		Currency     string    `json:"currency"`
		TotalAmount  int64     `json:"total_amount_minor_units"`
		Status       string    `json:"status"`
		SourceSystem string    `json:"source_system"`
		Lines        []struct {
			ID             string         `json:"id"`
			BillableItemID string         `json:"billable_item_id"`
			Description    string         `json:"description"`
			Quantity       int64          `json:"quantity"`
			UnitPrice      int64          `json:"unit_price_minor_units"`
			DiscountAmount int64          `json:"discount_amount_minor_units"`
			TaxAmount      int64          `json:"tax_amount_minor_units"`
			LineTotal      int64          `json:"line_total_minor_units"`
			SourceRef      string         `json:"source_ref"`
			Pricing        map[string]any `json:"pricing_snapshot"`
		} `json:"lines"`
	}

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := optionalUUID("invoice id", req.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, err := parseUUID("invoice tenant id", req.TenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	customerID, err := parseUUID("invoice customer id", req.CustomerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	contractID, err := parseUUID("invoice contract id", req.ContractID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	period, err := valueobject.NewBillingPeriod(req.PeriodStart, req.PeriodEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	totalAmount, err := valueobject.NewMoney(req.Currency, req.TotalAmount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	invoice := billing.Invoice{
		ID:           id,
		TenantID:     tenantID,
		CustomerID:   customerID,
		ContractID:   contractID,
		ExternalID:   req.ExternalID,
		Number:       req.Number,
		Period:       period,
		IssuedAt:     req.IssuedAt,
		DueAt:        req.DueAt,
		TotalAmount:  totalAmount,
		Status:       billing.InvoiceStatus(req.Status),
		SourceSystem: req.SourceSystem,
	}

	lines := make([]billing.InvoiceLine, 0, len(req.Lines))
	for _, l := range req.Lines {
		lineID, err := optionalUUID("invoice line id", l.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		itemID, err := optionalUUID("invoice line billable item id", l.BillableItemID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		unitPrice, err := valueobject.NewMoney(req.Currency, l.UnitPrice)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		discountAmount, err := valueobject.NewMoney(req.Currency, l.DiscountAmount)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		taxAmount, err := valueobject.NewMoney(req.Currency, l.TaxAmount)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		lineTotal, err := valueobject.NewMoney(req.Currency, l.LineTotal)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		lines = append(lines, billing.InvoiceLine{
			ID:              lineID,
			InvoiceID:       id,
			BillableItemID:  itemID,
			Description:     l.Description,
			Quantity:        l.Quantity,
			UnitPrice:       unitPrice,
			DiscountAmount:  discountAmount,
			TaxAmount:       taxAmount,
			LineTotal:       lineTotal,
			SourceRef:       l.SourceRef,
			PricingSnapshot: l.Pricing,
		})
	}

	if err := h.ingestionCommands.IngestInvoice(r.Context(), invoice, lines); err != nil {
		writeIngestionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ingestionResponse{
		Status:       "ok",
		Invoices:     1,
		InvoiceLines: len(lines),
	})
}

func writeIngestionError(w http.ResponseWriter, err error) {
	if isIngestionValidationError(err) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeError(w, http.StatusInternalServerError, err.Error())
}

func isIngestionValidationError(err error) bool {
	for _, target := range ingestionValidationErrors {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}
