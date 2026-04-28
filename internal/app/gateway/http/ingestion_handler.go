package http

import (
	"context"
	"net/http"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

type ingestionCommands interface {
	IngestUsageRecords(ctx context.Context, records []*billing.UsageRecord) error
	IngestInvoice(ctx context.Context, inv *billing.Invoice, lines []billing.InvoiceLine) error
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

	records := make([]*billing.UsageRecord, 0, len(req.Records))
	for _, rec := range req.Records {
		id, _ := uuid.Parse(rec.ID)
		tenantID, _ := uuid.Parse(rec.TenantID)
		customerID, _ := uuid.Parse(rec.CustomerID)
		contractID, _ := uuid.Parse(rec.ContractID)
		billableItemID, _ := uuid.Parse(rec.BillableItemID)

		records = append(records, &billing.UsageRecord{
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

	id, _ := uuid.Parse(req.ID)
	tenantID, _ := uuid.Parse(req.TenantID)
	customerID, _ := uuid.Parse(req.CustomerID)
	contractID, _ := uuid.Parse(req.ContractID)

	period, err := valueobject.NewBillingPeriod(req.PeriodStart, req.PeriodEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid billing period")
		return
	}

	totalAmount, err := valueobject.NewMoney(req.Currency, req.TotalAmount)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid total amount")
		return
	}

	inv := &billing.Invoice{
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
		lineID, _ := uuid.Parse(l.ID)
		itemID, _ := uuid.Parse(l.BillableItemID)
		up, _ := valueobject.NewMoney(req.Currency, l.UnitPrice)
		da, _ := valueobject.NewMoney(req.Currency, l.DiscountAmount)
		ta, _ := valueobject.NewMoney(req.Currency, l.TaxAmount)
		lt, _ := valueobject.NewMoney(req.Currency, l.LineTotal)

		lines = append(lines, billing.InvoiceLine{
			ID:              lineID,
			InvoiceID:       id,
			BillableItemID:  itemID,
			Description:     l.Description,
			Quantity:        l.Quantity,
			UnitPrice:       up,
			DiscountAmount:  da,
			TaxAmount:       ta,
			LineTotal:       lt,
			SourceRef:       l.SourceRef,
			PricingSnapshot: l.Pricing,
		})
	}

	if err := h.ingestionCommands.IngestInvoice(r.Context(), inv, lines); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
