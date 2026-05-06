package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrEventTypeRequired = errors.New("event type is required")
	ErrTopicRequired     = errors.New("topic is required")
	ErrTenantRequired    = errors.New("tenant id is required")
	ErrPayloadRequired   = errors.New("payload is required")
)

// Envelope is the stable JSON wrapper used in outbox rows and Kafka messages.
type Envelope struct {
	EventID      uuid.UUID       `json:"event_id"`
	Type         string          `json:"type"`
	Topic        string          `json:"topic"`
	Version      int             `json:"version"`
	OccurredAt   time.Time       `json:"occurred_at"`
	TenantID     uuid.UUID       `json:"tenant_id"`
	ContractID   uuid.UUID       `json:"contract_id,omitempty"`
	TraceID      string          `json:"trace_id,omitempty"`
	PartitionKey string          `json:"partition_key"`
	Payload      json.RawMessage `json:"payload"`
}

// NewEnvelope builds a validated event envelope with JSON-encoded payload.
func NewEnvelope(command NewEnvelopeCommand) (Envelope, error) {
	if command.EventID == uuid.Nil {
		command.EventID = uuid.New()
	}

	if command.OccurredAt.IsZero() {
		command.OccurredAt = time.Now()
	}

	payload, err := json.Marshal(command.Payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal payload: %w", err)
	}

	envelope := Envelope{
		EventID:      command.EventID,
		Type:         strings.TrimSpace(command.Type),
		Topic:        strings.TrimSpace(command.Topic),
		Version:      command.Version,
		OccurredAt:   command.OccurredAt.UTC(),
		TenantID:     command.TenantID,
		ContractID:   command.ContractID,
		TraceID:      strings.TrimSpace(command.TraceID),
		PartitionKey: strings.TrimSpace(command.PartitionKey),
		Payload:      payload,
	}

	if envelope.Version == 0 {
		envelope.Version = 1
	}

	if envelope.PartitionKey == "" {
		envelope.PartitionKey = defaultPartitionKey(envelope.TenantID, envelope.ContractID)
	}

	if err := envelope.Validate(); err != nil {
		return Envelope{}, err
	}

	return envelope, nil
}

// NewEnvelopeCommand describes an application event before serialization.
type NewEnvelopeCommand struct {
	EventID      uuid.UUID
	Type         string
	Topic        string
	Version      int
	OccurredAt   time.Time
	TenantID     uuid.UUID
	ContractID   uuid.UUID
	TraceID      string
	PartitionKey string
	Payload      any
}

// Validate checks the envelope fields required for idempotency and routing.
func (e Envelope) Validate() error {
	if strings.TrimSpace(e.Type) == "" {
		return ErrEventTypeRequired
	}

	if strings.TrimSpace(e.Topic) == "" {
		return ErrTopicRequired
	}

	if e.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if len(e.Payload) == 0 || string(e.Payload) == "null" {
		return ErrPayloadRequired
	}

	return nil
}

// MarshalJSONPayload returns the complete envelope JSON body.
func (e Envelope) MarshalJSONPayload() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}

	return payload, nil
}

func defaultPartitionKey(tenantID, contractID uuid.UUID) string {
	if contractID != uuid.Nil {
		return contractID.String()
	}

	return tenantID.String()
}
