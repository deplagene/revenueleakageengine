package ingestion

import (
	"context"
	"fmt"

	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
)

func (c *Commands) publishUsageRecordsIngested(ctx context.Context, records []billing.UsageRecord) error {
	if c.events == nil {
		return nil
	}

	envelopes := make([]appevent.Envelope, 0, len(records))
	for index, record := range records {
		envelope, err := appevent.NewEnvelope(appevent.NewEnvelopeCommand{
			Type:       appevent.TypeUsageRecordsIngested,
			Topic:      appevent.TopicUsageRecordsV1,
			OccurredAt: c.now(),
			TenantID:   record.TenantID,
			ContractID: record.ContractID,
			TraceID:    record.TraceID,
			Payload:    usageRecordPayload(record),
		})
		if err != nil {
			return fmt.Errorf("usage record[%d]: %w", index, err)
		}

		envelopes = append(envelopes, envelope)
	}

	return c.events.AppendBatch(ctx, envelopes)
}

func (c *Commands) publishInvoiceIngested(
	ctx context.Context,
	invoice billing.Invoice,
	lines []billing.InvoiceLine,
) error {
	if c.events == nil {
		return nil
	}

	envelope, err := appevent.NewEnvelope(appevent.NewEnvelopeCommand{
		Type:       appevent.TypeBillingInvoiceIngested,
		Topic:      appevent.TopicBillingInvoicesV1,
		OccurredAt: c.now(),
		TenantID:   invoice.TenantID,
		ContractID: invoice.ContractID,
		Payload:    invoicePayload(invoice, lines),
	})
	if err != nil {
		return err
	}

	return c.events.Append(ctx, envelope)
}

func usageRecordPayload(record billing.UsageRecord) appevent.UsageRecordPayload {
	return appevent.UsageRecordPayload{
		ID:             record.ID,
		TenantID:       record.TenantID,
		CustomerID:     record.CustomerID,
		ContractID:     record.ContractID,
		BillableItemID: record.BillableItemID,
		ExternalID:     record.ExternalID,
		UsageTime:      record.UsageTime,
		Quantity:       record.Quantity,
		Unit:           record.Unit,
		SourceSystem:   record.SourceSystem,
		TraceID:        record.TraceID,
		Metadata:       record.Metadata,
	}
}

func invoicePayload(invoice billing.Invoice, lines []billing.InvoiceLine) appevent.InvoicePayload {
	payloadLines := make([]appevent.InvoiceLinePayload, 0, len(lines))
	for _, line := range lines {
		payloadLines = append(payloadLines, appevent.InvoiceLinePayload{
			ID:              line.ID,
			BillableItemID:  line.BillableItemID,
			Description:     line.Description,
			Quantity:        line.Quantity,
			UnitPrice:       moneyPayload(line.UnitPrice),
			DiscountAmount:  moneyPayload(line.DiscountAmount),
			TaxAmount:       moneyPayload(line.TaxAmount),
			LineTotal:       moneyPayload(line.LineTotal),
			SourceRef:       line.SourceRef,
			PricingSnapshot: line.PricingSnapshot,
		})
	}

	return appevent.InvoicePayload{
		ID:           invoice.ID,
		TenantID:     invoice.TenantID,
		CustomerID:   invoice.CustomerID,
		ContractID:   invoice.ContractID,
		ExternalID:   invoice.ExternalID,
		Number:       invoice.Number,
		Period:       billingPeriodPayload(invoice.Period),
		IssuedAt:     invoice.IssuedAt,
		DueAt:        invoice.DueAt,
		TotalAmount:  moneyPayload(invoice.TotalAmount),
		Status:       string(invoice.Status),
		SourceSystem: invoice.SourceSystem,
		Lines:        payloadLines,
	}
}

func billingPeriodPayload(period valueobject.BillingPeriod) appevent.BillingPeriodPayload {
	return appevent.BillingPeriodPayload{
		Start: period.Start,
		End:   period.End,
	}
}

func moneyPayload(money valueobject.Money) appevent.MoneyPayload {
	return appevent.MoneyPayload{
		Currency:   money.Currency,
		MinorUnits: money.MinorUnits,
	}
}
