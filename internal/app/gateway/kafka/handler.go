// Package kafka contains Kafka delivery handlers that map v1 event envelopes to
// application use cases.
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	"github.com/deplagene/revenueleakageengine/internal/app/outbox"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	platformkafka "github.com/deplagene/revenueleakageengine/internal/platform/kafka"
	"github.com/google/uuid"
)

const defaultHandlerName = "revenue-leakage-engine.kafka.gateway.v1"

var (
	ErrIngestionCommandsRequired    = errors.New("ingestion commands are required")
	ErrReconciliationRunnerRequired = errors.New("reconciliation runner is required")
	ErrInboxRequired                = errors.New("inbox store is required")
	ErrEventIDRequired              = errors.New("event id is required")
)

type ingestionCommands interface {
	IngestUsageRecords(ctx context.Context, records []billing.UsageRecord) error
	IngestInvoice(ctx context.Context, invoice billing.Invoice, lines []billing.InvoiceLine) error
}

type reconciliationRunner interface {
	RunRevenueLeakageCheck(
		ctx context.Context,
		cmd appreconciliation.RunRevenueLeakageCheckCommand,
	) (appreconciliation.RunRevenueLeakageCheckResult, error)
}

type inboxStore interface {
	AlreadyProcessed(ctx context.Context, eventID uuid.UUID, handler string) (bool, error)
	MarkProcessed(ctx context.Context, record outbox.InboxRecord) error
}

type Handler struct {
	ingestion      ingestionCommands
	reconciliation reconciliationRunner
	inbox          inboxStore
	handlerName    string
}

func NewHandler(
	ingestion ingestionCommands,
	reconciliation reconciliationRunner,
	inbox inboxStore,
) (*Handler, error) {
	if ingestion == nil {
		return nil, ErrIngestionCommandsRequired
	}

	if reconciliation == nil {
		return nil, ErrReconciliationRunnerRequired
	}

	if inbox == nil {
		return nil, ErrInboxRequired
	}

	return &Handler{
		ingestion:      ingestion,
		reconciliation: reconciliation,
		inbox:          inbox,
		handlerName:    defaultHandlerName,
	}, nil
}

func (h *Handler) Handle(ctx context.Context, msg platformkafka.Message) error {
	envelope, err := decodeEnvelope(msg.Value)
	if err != nil {
		return err
	}

	if envelope.EventID == uuid.Nil {
		return ErrEventIDRequired
	}

	processed, err := h.inbox.AlreadyProcessed(ctx, envelope.EventID, h.handlerName)
	if err != nil {
		return fmt.Errorf("check inbox: %w", err)
	}

	if processed {
		return nil
	}

	if err := h.dispatch(ctx, envelope); err != nil {
		return err
	}

	if err := h.inbox.MarkProcessed(ctx, outbox.InboxRecord{
		EventID:     envelope.EventID,
		Handler:     h.handlerName,
		Topic:       envelope.Topic,
		SourceKey:   sourceKey(envelope, msg),
		ProcessedAt: envelope.OccurredAt,
	}); err != nil {
		return fmt.Errorf("mark inbox processed: %w", err)
	}

	return nil
}

func (h *Handler) dispatch(ctx context.Context, envelope appevent.Envelope) error {
	switch envelope.Type {
	case appevent.TypeUsageRecordReceived:
		return h.handleUsageRecord(ctx, envelope)
	case appevent.TypeBillingInvoiceReceived:
		return h.handleInvoice(ctx, envelope)
	case appevent.TypeReconciliationRunRequested:
		return h.handleReconciliationRequested(ctx, envelope)
	default:
		return nil
	}
}

func (h *Handler) handleUsageRecord(ctx context.Context, envelope appevent.Envelope) error {
	var payload appevent.UsageRecordPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode usage record payload: %w", err)
	}

	return h.ingestion.IngestUsageRecords(ctx, []billing.UsageRecord{
		{
			ID:             payload.ID,
			TenantID:       payload.TenantID,
			CustomerID:     payload.CustomerID,
			ContractID:     payload.ContractID,
			BillableItemID: payload.BillableItemID,
			ExternalID:     payload.ExternalID,
			UsageTime:      payload.UsageTime,
			Quantity:       payload.Quantity,
			Unit:           payload.Unit,
			SourceSystem:   payload.SourceSystem,
			TraceID:        firstNonEmpty(payload.TraceID, envelope.TraceID),
			Metadata:       payload.Metadata,
		},
	})
}

func (h *Handler) handleInvoice(ctx context.Context, envelope appevent.Envelope) error {
	var payload appevent.InvoicePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode invoice payload: %w", err)
	}

	invoice, lines, err := invoiceFromPayload(payload)
	if err != nil {
		return err
	}

	return h.ingestion.IngestInvoice(ctx, invoice, lines)
}

func (h *Handler) handleReconciliationRequested(ctx context.Context, envelope appevent.Envelope) error {
	var payload appevent.ReconciliationRunRequestedPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode reconciliation requested payload: %w", err)
	}

	period, err := valueobject.NewBillingPeriod(payload.Period.Start, payload.Period.End)
	if err != nil {
		return fmt.Errorf("build reconciliation period: %w", err)
	}

	var minimumLeakage valueobject.Money
	if payload.Currency != "" || payload.MinimumLeakageMinorUnits != 0 {
		minimumLeakage, err = valueobject.NewMoney(payload.Currency, payload.MinimumLeakageMinorUnits)
		if err != nil {
			return fmt.Errorf("build minimum leakage: %w", err)
		}
	}

	_, err = h.reconciliation.RunRevenueLeakageCheck(ctx, appreconciliation.RunRevenueLeakageCheckCommand{
		TenantID:             payload.TenantID,
		ContractID:           payload.ContractID,
		PeriodStart:          period.Start,
		PeriodEnd:            period.End,
		RunID:                payload.RunID,
		TraceID:              firstNonEmpty(payload.TraceID, envelope.TraceID),
		Currency:             payload.Currency,
		MinimumLeakageAmount: minimumLeakage,
	})
	if err != nil {
		return fmt.Errorf("run reconciliation: %w", err)
	}

	return nil
}

func invoiceFromPayload(payload appevent.InvoicePayload) (billing.Invoice, []billing.InvoiceLine, error) {
	totalAmount, err := moneyFromPayload(payload.TotalAmount)
	if err != nil {
		return billing.Invoice{}, nil, fmt.Errorf("build total amount: %w", err)
	}

	period, err := valueobject.NewBillingPeriod(payload.Period.Start, payload.Period.End)
	if err != nil {
		return billing.Invoice{}, nil, fmt.Errorf("build invoice period: %w", err)
	}

	invoice := billing.Invoice{
		ID:           payload.ID,
		TenantID:     payload.TenantID,
		CustomerID:   payload.CustomerID,
		ContractID:   payload.ContractID,
		ExternalID:   payload.ExternalID,
		Number:       payload.Number,
		Period:       period,
		IssuedAt:     payload.IssuedAt,
		DueAt:        payload.DueAt,
		TotalAmount:  totalAmount,
		Status:       billing.InvoiceStatus(payload.Status),
		SourceSystem: payload.SourceSystem,
	}

	lines := make([]billing.InvoiceLine, 0, len(payload.Lines))
	for index, payloadLine := range payload.Lines {
		line, err := invoiceLineFromPayload(payloadLine)
		if err != nil {
			return billing.Invoice{}, nil, fmt.Errorf("line[%d]: %w", index, err)
		}

		lines = append(lines, line)
	}

	return invoice, lines, nil
}

func invoiceLineFromPayload(payload appevent.InvoiceLinePayload) (billing.InvoiceLine, error) {
	unitPrice, err := moneyFromPayload(payload.UnitPrice)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build unit price: %w", err)
	}

	discountAmount, err := moneyFromPayload(payload.DiscountAmount)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build discount amount: %w", err)
	}

	taxAmount, err := moneyFromPayload(payload.TaxAmount)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build tax amount: %w", err)
	}

	lineTotal, err := moneyFromPayload(payload.LineTotal)
	if err != nil {
		return billing.InvoiceLine{}, fmt.Errorf("build line total: %w", err)
	}

	return billing.InvoiceLine{
		ID:              payload.ID,
		BillableItemID:  payload.BillableItemID,
		Description:     payload.Description,
		Quantity:        payload.Quantity,
		UnitPrice:       unitPrice,
		DiscountAmount:  discountAmount,
		TaxAmount:       taxAmount,
		LineTotal:       lineTotal,
		SourceRef:       payload.SourceRef,
		PricingSnapshot: payload.PricingSnapshot,
	}, nil
}

func moneyFromPayload(payload appevent.MoneyPayload) (valueobject.Money, error) {
	return valueobject.NewMoney(payload.Currency, payload.MinorUnits)
}

func decodeEnvelope(value []byte) (appevent.Envelope, error) {
	var envelope appevent.Envelope
	if err := json.Unmarshal(value, &envelope); err != nil {
		return appevent.Envelope{}, fmt.Errorf("decode event envelope: %w", err)
	}

	if err := envelope.Validate(); err != nil {
		return appevent.Envelope{}, err
	}

	return envelope, nil
}

func sourceKey(envelope appevent.Envelope, msg platformkafka.Message) string {
	if envelope.PartitionKey != "" {
		return envelope.PartitionKey
	}

	if len(msg.Key) > 0 {
		return string(msg.Key)
	}

	return envelope.EventID.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}

	return ""
}
