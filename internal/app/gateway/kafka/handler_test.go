package kafka

import (
	"context"
	"testing"
	"time"

	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	"github.com/deplagene/revenueleakageengine/internal/app/outbox"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	platformkafka "github.com/deplagene/revenueleakageengine/internal/platform/kafka"
	"github.com/google/uuid"
)

func TestHandlerMapsUsageRecordEventToIngestionCommand(t *testing.T) {
	t.Parallel()

	ingestion := &fakeIngestionCommands{}
	inbox := &fakeInbox{}
	handler, err := NewHandler(ingestion, &fakeReconciliationRunner{}, inbox)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	tenantID := uuid.New()
	envelope := mustEnvelope(t, appevent.NewEnvelopeCommand{
		Type:     appevent.TypeUsageRecordReceived,
		Topic:    appevent.TopicUsageRecordsV1,
		TenantID: tenantID,
		Payload: appevent.UsageRecordPayload{
			TenantID:       tenantID,
			CustomerID:     uuid.New(),
			ContractID:     uuid.New(),
			BillableItemID: uuid.New(),
			ExternalID:     "usage-1",
			UsageTime:      time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
			Quantity:       42,
			Unit:           "events",
			SourceSystem:   "metering",
		},
	})

	if err := handler.Handle(context.Background(), platformkafka.Message{Value: envelope}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if got := len(ingestion.usageRecords); got != 1 {
		t.Fatalf("usage record count = %d, want 1", got)
	}
	if ingestion.usageRecords[0].ExternalID != "usage-1" {
		t.Fatalf("external id = %q, want usage-1", ingestion.usageRecords[0].ExternalID)
	}
	if inbox.marked.EventID == uuid.Nil {
		t.Fatal("inbox was not marked")
	}
}

func TestHandlerSkipsAlreadyProcessedEvent(t *testing.T) {
	t.Parallel()

	ingestion := &fakeIngestionCommands{}
	inbox := &fakeInbox{processed: true}
	handler, err := NewHandler(ingestion, &fakeReconciliationRunner{}, inbox)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	tenantID := uuid.New()
	envelope := mustEnvelope(t, appevent.NewEnvelopeCommand{
		Type:     appevent.TypeUsageRecordReceived,
		Topic:    appevent.TopicUsageRecordsV1,
		TenantID: tenantID,
		Payload: appevent.UsageRecordPayload{
			TenantID:     tenantID,
			ExternalID:   "usage-1",
			SourceSystem: "metering",
		},
	})

	if err := handler.Handle(context.Background(), platformkafka.Message{Value: envelope}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if len(ingestion.usageRecords) != 0 {
		t.Fatalf("usage record count = %d, want 0", len(ingestion.usageRecords))
	}
}

func TestHandlerMapsReconciliationRequestedEvent(t *testing.T) {
	t.Parallel()

	reconciliation := &fakeReconciliationRunner{}
	handler, err := NewHandler(&fakeIngestionCommands{}, reconciliation, &fakeInbox{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	tenantID := uuid.New()
	contractID := uuid.New()
	envelope := mustEnvelope(t, appevent.NewEnvelopeCommand{
		Type:       appevent.TypeReconciliationRunRequested,
		Topic:      appevent.TopicReconciliationRunRequestedV1,
		TenantID:   tenantID,
		ContractID: contractID,
		Payload: appevent.ReconciliationRunRequestedPayload{
			TenantID:   tenantID,
			ContractID: contractID,
			Period: appevent.BillingPeriodPayload{
				Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
			},
			Currency:                 "USD",
			MinimumLeakageMinorUnits: 100,
		},
	})

	if err := handler.Handle(context.Background(), platformkafka.Message{Value: envelope}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if reconciliation.cmd.TenantID != tenantID {
		t.Fatalf("tenant id = %s, want %s", reconciliation.cmd.TenantID, tenantID)
	}
	if reconciliation.cmd.MinimumLeakageAmount.MinorUnits != 100 {
		t.Fatalf("minimum leakage = %d, want 100", reconciliation.cmd.MinimumLeakageAmount.MinorUnits)
	}
}

func mustEnvelope(t *testing.T, command appevent.NewEnvelopeCommand) []byte {
	t.Helper()

	envelope, err := appevent.NewEnvelope(command)
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	encoded, err := envelope.MarshalJSONPayload()
	if err != nil {
		t.Fatalf("MarshalJSONPayload() error = %v", err)
	}

	return encoded
}

type fakeIngestionCommands struct {
	usageRecords []billing.UsageRecord
	invoice      billing.Invoice
	invoiceLines []billing.InvoiceLine
}

func (f *fakeIngestionCommands) IngestUsageRecords(
	_ context.Context,
	records []billing.UsageRecord,
) error {
	f.usageRecords = append([]billing.UsageRecord{}, records...)
	return nil
}

func (f *fakeIngestionCommands) IngestInvoice(
	_ context.Context,
	invoice billing.Invoice,
	lines []billing.InvoiceLine,
) error {
	f.invoice = invoice
	f.invoiceLines = append([]billing.InvoiceLine{}, lines...)
	return nil
}

type fakeReconciliationRunner struct {
	cmd appreconciliation.RunRevenueLeakageCheckCommand
}

func (f *fakeReconciliationRunner) RunRevenueLeakageCheck(
	_ context.Context,
	cmd appreconciliation.RunRevenueLeakageCheckCommand,
) (appreconciliation.RunRevenueLeakageCheckResult, error) {
	f.cmd = cmd
	return appreconciliation.RunRevenueLeakageCheckResult{}, nil
}

type fakeInbox struct {
	processed bool
	marked    outbox.InboxRecord
}

func (f *fakeInbox) AlreadyProcessed(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return f.processed, nil
}

func (f *fakeInbox) MarkProcessed(_ context.Context, record outbox.InboxRecord) error {
	f.marked = record
	return nil
}
