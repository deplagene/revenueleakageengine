package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type recordingIngestionCommands struct {
	usageRecords []billing.UsageRecord
	invoice      billing.Invoice
	invoiceLines []billing.InvoiceLine
	err          error
}

func (c *recordingIngestionCommands) IngestUsageRecords(
	_ context.Context,
	records []billing.UsageRecord,
) error {
	c.usageRecords = records
	return c.err
}

func (c *recordingIngestionCommands) IngestInvoice(
	_ context.Context,
	invoice billing.Invoice,
	lines []billing.InvoiceLine,
) error {
	c.invoice = invoice
	c.invoiceLines = lines
	return c.err
}

func TestHandlerIngestUsage(t *testing.T) {
	t.Parallel()

	commands := &recordingIngestionCommands{}
	handler := newIngestionTestHandler(t, commands)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/ingest/usage",
		strings.NewReader(`{
			"records": [
				{
					"tenant_id": "11111111-1111-1111-1111-111111111111",
					"customer_id": "22222222-2222-2222-2222-222222222222",
					"contract_id": "33333333-3333-3333-3333-333333333333",
					"billable_item_id": "44444444-4444-4444-4444-444444444444",
					"external_id": "usage-1",
					"usage_time": "2026-04-10T12:00:00Z",
					"quantity": 100,
					"unit": "api_calls",
					"source_system": "metering"
				}
			]
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if got := len(commands.usageRecords); got != 1 {
		t.Fatalf("received usage record count = %d, want 1", got)
	}

	if commands.usageRecords[0].TenantID != uuid.MustParse("11111111-1111-1111-1111-111111111111") {
		t.Fatalf("tenant id = %s, want request tenant", commands.usageRecords[0].TenantID)
	}

	var payload ingestionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.UsageRecords != 1 {
		t.Fatalf("usage records response count = %d, want 1", payload.UsageRecords)
	}
}

func TestHandlerIngestUsageRejectsInvalidUUID(t *testing.T) {
	t.Parallel()

	commands := &recordingIngestionCommands{}
	handler := newIngestionTestHandler(t, commands)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/ingest/usage",
		strings.NewReader(`{
			"records": [
				{
					"tenant_id": "bad-uuid",
					"customer_id": "22222222-2222-2222-2222-222222222222",
					"contract_id": "33333333-3333-3333-3333-333333333333",
					"billable_item_id": "44444444-4444-4444-4444-444444444444",
					"external_id": "usage-1",
					"usage_time": "2026-04-10T12:00:00Z",
					"quantity": 100,
					"unit": "api_calls",
					"source_system": "metering"
				}
			]
		}`),
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, stdhttp.StatusBadRequest)
	}

	if got := len(commands.usageRecords); got != 0 {
		t.Fatalf("received usage record count = %d, want 0", got)
	}
}

func TestHandlerIngestInvoices(t *testing.T) {
	t.Parallel()

	commands := &recordingIngestionCommands{}
	handler := newIngestionTestHandler(t, commands)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/ingest/invoices",
		strings.NewReader(`{
			"tenant_id": "11111111-1111-1111-1111-111111111111",
			"customer_id": "22222222-2222-2222-2222-222222222222",
			"contract_id": "33333333-3333-3333-3333-333333333333",
			"external_id": "invoice-1",
			"number": "INV-001",
			"period_start": "2026-04-01T00:00:00Z",
			"period_end": "2026-05-01T00:00:00Z",
			"issued_at": "2026-05-01T09:00:00Z",
			"due_at": "2026-05-15T00:00:00Z",
			"currency": "USD",
			"total_amount_minor_units": 10000,
			"status": "issued",
			"source_system": "stripe",
			"lines": [
				{
					"billable_item_id": "44444444-4444-4444-4444-444444444444",
					"description": "Platform subscription",
					"quantity": 1,
					"unit_price_minor_units": 10000,
					"discount_amount_minor_units": 0,
					"tax_amount_minor_units": 0,
					"line_total_minor_units": 10000
				}
			]
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusOK, response.Body.String())
	}

	if commands.invoice.ExternalID != "invoice-1" {
		t.Fatalf("invoice external id = %q, want invoice-1", commands.invoice.ExternalID)
	}

	if got := len(commands.invoiceLines); got != 1 {
		t.Fatalf("invoice line count = %d, want 1", got)
	}
}

func TestHandlerIngestInvoicesMapsValidationErrorsToBadRequest(t *testing.T) {
	t.Parallel()

	commands := &recordingIngestionCommands{
		err: billing.ErrInvoiceNumberRequired,
	}
	handler := newIngestionTestHandler(t, commands)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/ingest/invoices",
		strings.NewReader(`{
			"tenant_id": "11111111-1111-1111-1111-111111111111",
			"customer_id": "22222222-2222-2222-2222-222222222222",
			"contract_id": "33333333-3333-3333-3333-333333333333",
			"external_id": "invoice-1",
			"number": "INV-001",
			"period_start": "2026-04-01T00:00:00Z",
			"period_end": "2026-05-01T00:00:00Z",
			"issued_at": "2026-05-01T09:00:00Z",
			"currency": "USD",
			"total_amount_minor_units": 10000,
			"status": "issued",
			"source_system": "stripe",
			"lines": [
				{
					"description": "Platform subscription",
					"quantity": 1,
					"unit_price_minor_units": 10000,
					"discount_amount_minor_units": 0,
					"tax_amount_minor_units": 0,
					"line_total_minor_units": 10000
				}
			]
		}`),
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, stdhttp.StatusBadRequest)
	}
}

func newIngestionTestHandler(t *testing.T, commands *recordingIngestionCommands) *Handler {
	t.Helper()

	handler, err := NewHandler(
		&noopReconciliationRunner{},
		&stubCaseQueries{},
		&stubCaseCommands{},
		&noopContractQueries{},
		&noopContractCommands{},
		commands,
	)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	return handler
}
