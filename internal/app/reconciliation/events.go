package reconciliation

import (
	"context"
	"fmt"

	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
)

func (w *RevenueLeakageWorkflow) publishReconciliationCompleted(
	ctx context.Context,
	cmd RunRevenueLeakageCheckCommand,
	result RunRevenueLeakageCheckResult,
	traceID string,
) error {
	if w.events == nil {
		return nil
	}

	reconciliationResult := result.ReconciliationResult
	envelopes := make([]appevent.Envelope, 0, 1+len(reconciliationResult.Cases))

	completed, err := appevent.NewEnvelope(appevent.NewEnvelopeCommand{
		Type:       appevent.TypeReconciliationRunCompleted,
		Topic:      appevent.TopicReconciliationRunCompletedV1,
		OccurredAt: w.now(),
		TenantID:   cmd.TenantID,
		ContractID: cmd.ContractID,
		TraceID:    traceID,
		Payload: appevent.ReconciliationRunCompletedPayload{
			RunID:            reconciliationResult.RunID,
			TenantID:         cmd.TenantID,
			ContractID:       cmd.ContractID,
			ExpectedEntryID:  result.ExpectedEntry,
			ActualEntryCount: result.ActualEntryCount,
			DiffCount:        reconciliationResult.DiffCount,
			CaseCount:        reconciliationResult.CaseCount,
			LeakageAmount:    moneyPayload(reconciliationResult.LeakageAmount),
			TraceID:          traceID,
		},
	})
	if err != nil {
		return fmt.Errorf("build completed event: %w", err)
	}
	envelopes = append(envelopes, completed)

	for index, item := range reconciliationResult.Cases {
		envelope, err := leakageCaseCreatedEnvelope(w, item)
		if err != nil {
			return fmt.Errorf("case[%d]: %w", index, err)
		}

		envelopes = append(envelopes, envelope)
	}

	return w.events.AppendBatch(ctx, envelopes)
}

func leakageCaseCreatedEnvelope(w *RevenueLeakageWorkflow, item leakage.Case) (appevent.Envelope, error) {
	return appevent.NewEnvelope(appevent.NewEnvelopeCommand{
		Type:       appevent.TypeLeakageCaseCreated,
		Topic:      appevent.TopicLeakageCaseCreatedV1,
		OccurredAt: w.now(),
		TenantID:   item.TenantID,
		ContractID: item.ContractID,
		TraceID:    item.TraceID,
		Payload: appevent.LeakageCaseCreatedPayload{
			CaseID:              item.ID,
			TenantID:            item.TenantID,
			CustomerID:          item.CustomerID,
			ContractID:          item.ContractID,
			ReconciliationRunID: item.ReconciliationRunID,
			Type:                string(item.Type),
			Severity:            string(item.Severity),
			Status:              string(item.Status),
			LeakageAmount:       moneyPayload(item.LeakageAmount),
			TraceID:             item.TraceID,
		},
	})
}

func moneyPayload(money valueobject.Money) appevent.MoneyPayload {
	return appevent.MoneyPayload{
		Currency:   money.Currency,
		MinorUnits: money.MinorUnits,
	}
}
