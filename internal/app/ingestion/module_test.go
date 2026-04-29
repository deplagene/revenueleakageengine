package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

func TestNewCommandsRequiresStore(t *testing.T) {
	t.Parallel()

	_, err := NewCommands(nil)
	if !errors.Is(err, ErrStoreRequired) {
		t.Fatalf("NewCommands(nil) error = %v, want %v", err, ErrStoreRequired)
	}
}

func TestCommandsIngestUsageRecordsNormalizesAndGeneratesIDs(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	commands, err := NewCommands(store)
	if err != nil {
		t.Fatalf("NewCommands() error = %v", err)
	}

	record := validUsageRecord()
	record.ID = uuid.Nil
	record.ExternalID = " usage-1 "
	record.SourceSystem = " metering "
	record.Metadata = nil

	if err := commands.IngestUsageRecords(context.Background(), []billing.UsageRecord{record}); err != nil {
		t.Fatalf("IngestUsageRecords() error = %v", err)
	}

	if got := len(store.usageRecords); got != 1 {
		t.Fatalf("stored usage record count = %d, want 1", got)
	}

	stored := store.usageRecords[0]
	if stored.ID == uuid.Nil {
		t.Fatal("usage record id was not generated")
	}

	if stored.ExternalID != "usage-1" {
		t.Fatalf("external id = %q, want %q", stored.ExternalID, "usage-1")
	}

	if stored.SourceSystem != "metering" {
		t.Fatalf("source system = %q, want %q", stored.SourceSystem, "metering")
	}

	if stored.Metadata == nil {
		t.Fatal("metadata is nil")
	}
}

func TestCommandsIngestUsageRecordsValidatesBeforeWriting(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	commands, err := NewCommands(store)
	if err != nil {
		t.Fatalf("NewCommands() error = %v", err)
	}

	valid := validUsageRecord()
	invalid := validUsageRecord()
	invalid.ExternalID = ""

	err = commands.IngestUsageRecords(context.Background(), []billing.UsageRecord{valid, invalid})
	if !errors.Is(err, billing.ErrExternalIDRequired) {
		t.Fatalf("IngestUsageRecords() error = %v, want %v", err, billing.ErrExternalIDRequired)
	}

	if got := len(store.usageRecords); got != 0 {
		t.Fatalf("stored usage record count = %d, want 0", got)
	}
}

func TestCommandsIngestInvoiceNormalizesAndGeneratesIDs(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	commands, err := NewCommands(store)
	if err != nil {
		t.Fatalf("NewCommands() error = %v", err)
	}

	invoice := validInvoice(t)
	invoice.ID = uuid.Nil
	line := validInvoiceLine(invoice.ID)
	line.ID = uuid.Nil
	line.InvoiceID = uuid.Nil
	line.PricingSnapshot = nil

	if err := commands.IngestInvoice(context.Background(), invoice, []billing.InvoiceLine{line}); err != nil {
		t.Fatalf("IngestInvoice() error = %v", err)
	}

	if store.invoice.ID == uuid.Nil {
		t.Fatal("invoice id was not generated")
	}

	if got := len(store.invoiceLines); got != 1 {
		t.Fatalf("stored invoice line count = %d, want 1", got)
	}

	storedLine := store.invoiceLines[0]
	if storedLine.ID == uuid.Nil {
		t.Fatal("invoice line id was not generated")
	}

	if storedLine.InvoiceID != store.invoice.ID {
		t.Fatalf("invoice line invoice id = %s, want %s", storedLine.InvoiceID, store.invoice.ID)
	}

	if storedLine.PricingSnapshot == nil {
		t.Fatal("pricing snapshot is nil")
	}
}

type recordingStore struct {
	usageRecords []billing.UsageRecord
	invoice      billing.Invoice
	invoiceLines []billing.InvoiceLine
}

func (s *recordingStore) UpsertUsageRecords(_ context.Context, records []billing.UsageRecord) error {
	s.usageRecords = append([]billing.UsageRecord{}, records...)
	return nil
}

func (s *recordingStore) UpsertInvoice(
	_ context.Context,
	invoice billing.Invoice,
	lines []billing.InvoiceLine,
) error {
	s.invoice = invoice
	s.invoiceLines = append([]billing.InvoiceLine{}, lines...)
	return nil
}

func (s *recordingStore) ListUsageRecordsForContractPeriod(
	ctx context.Context,
	query ContractPeriodQuery,
) ([]billing.UsageRecord, error) {
	return nil, nil
}

func (s *recordingStore) ListInvoicesForContractPeriod(
	ctx context.Context,
	query ContractPeriodQuery,
) ([]billing.Invoice, []billing.InvoiceLine, error) {
	return nil, nil, nil
}

func validUsageRecord() billing.UsageRecord {
	return billing.UsageRecord{
		ID:             uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		TenantID:       uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		CustomerID:     uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		ContractID:     uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		BillableItemID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		ExternalID:     "usage-1",
		UsageTime:      time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC),
		Quantity:       100,
		Unit:           "api_calls",
		SourceSystem:   "metering",
		TraceID:        "trace-1",
		Metadata:       map[string]any{},
	}
}

func validInvoice(t *testing.T) billing.Invoice {
	t.Helper()

	period, err := valueobject.NewBillingPeriod(
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewBillingPeriod() error = %v", err)
	}

	return billing.Invoice{
		ID:           uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		TenantID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		CustomerID:   uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		ContractID:   uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		ExternalID:   "invoice-1",
		Number:       "INV-001",
		Period:       period,
		IssuedAt:     time.Date(2026, time.May, 1, 9, 0, 0, 0, time.UTC),
		DueAt:        time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC),
		TotalAmount:  valueobject.MustMoney("USD", 10000),
		Status:       billing.InvoiceStatusIssued,
		SourceSystem: "stripe",
	}
}

func validInvoiceLine(invoiceID uuid.UUID) billing.InvoiceLine {
	return billing.InvoiceLine{
		ID:             uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		InvoiceID:      invoiceID,
		BillableItemID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Description:    "Platform subscription",
		Quantity:       1,
		UnitPrice:      valueobject.MustMoney("USD", 10000),
		DiscountAmount: valueobject.MustMoney("USD", 0),
		TaxAmount:      valueobject.MustMoney("USD", 0),
		LineTotal:      valueobject.MustMoney("USD", 10000),
		SourceRef:      "stripe:line-1",
		PricingSnapshot: map[string]any{
			"source": "test",
		},
	}
}
