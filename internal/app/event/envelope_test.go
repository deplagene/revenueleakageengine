package event

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewEnvelopeDefaultsIDVersionTimestampAndPartitionKey(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	contractID := uuid.New()

	envelope, err := NewEnvelope(NewEnvelopeCommand{
		Type:       TypeReconciliationRunRequested,
		Topic:      TopicReconciliationRunRequestedV1,
		TenantID:   tenantID,
		ContractID: contractID,
		Payload: ReconciliationRunRequestedPayload{
			TenantID:   tenantID,
			ContractID: contractID,
			Period: BillingPeriodPayload{
				Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
			},
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	if envelope.EventID == uuid.Nil {
		t.Fatal("event id was not generated")
	}

	if envelope.Version != 1 {
		t.Fatalf("version = %d, want 1", envelope.Version)
	}

	if envelope.OccurredAt.IsZero() {
		t.Fatal("occurred_at was not generated")
	}

	if envelope.PartitionKey != contractID.String() {
		t.Fatalf("partition key = %q, want %q", envelope.PartitionKey, contractID)
	}

	var payload ReconciliationRunRequestedPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("payload json error = %v", err)
	}
	if payload.TenantID != tenantID {
		t.Fatalf("payload tenant id = %s, want %s", payload.TenantID, tenantID)
	}
}

func TestNewEnvelopeRejectsMissingTenant(t *testing.T) {
	t.Parallel()

	_, err := NewEnvelope(NewEnvelopeCommand{
		Type:    TypeBillingInvoiceReceived,
		Topic:   TopicBillingInvoicesV1,
		Payload: InvoicePayload{},
	})
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("NewEnvelope() error = %v, want %v", err, ErrTenantRequired)
	}
}

func TestEnvelopeMarshalJSONPayloadIncludesEnvelopeFields(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	envelope, err := NewEnvelope(NewEnvelopeCommand{
		EventID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Type:       TypeUsageRecordReceived,
		Topic:      TopicUsageRecordsV1,
		OccurredAt: time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC),
		TenantID:   tenantID,
		Payload: UsageRecordPayload{
			TenantID:     tenantID,
			ExternalID:   "usage-1",
			SourceSystem: "metering",
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	encoded, err := envelope.MarshalJSONPayload()
	if err != nil {
		t.Fatalf("MarshalJSONPayload() error = %v", err)
	}

	var decoded Envelope
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}

	if decoded.EventID != envelope.EventID {
		t.Fatalf("event id = %s, want %s", decoded.EventID, envelope.EventID)
	}
}
