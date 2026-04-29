package http

import (
	"fmt"
	stdhttp "net/http"
	"time"

	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

type runReconciliationRequest struct {
	TenantID                 string               `json:"tenant_id"`
	ContractID               string               `json:"contract_id"`
	Currency                 string               `json:"currency"`
	TraceID                  string               `json:"trace_id"`
	MinimumLeakageMinorUnits int64                `json:"minimum_leakage_minor_units"`
	Period                   billingPeriodRequest `json:"period"`
}

type billingPeriodRequest struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type runReconciliationResponse struct {
	RunID                   string                  `json:"run_id"`
	ExpectedEntryID         string                  `json:"expected_entry_id"`
	ActualEntryCount        int                     `json:"actual_entry_count"`
	DiffCount               int                     `json:"diff_count"`
	CaseCount               int                     `json:"case_count"`
	LeakageAmountMinorUnits int64                   `json:"leakage_amount_minor_units"`
	Currency                string                  `json:"currency"`
	Cases                   []runReconciliationCase `json:"cases"`
}

type runReconciliationCase struct {
	ID                         string `json:"id"`
	Type                       string `json:"type"`
	Severity                   string `json:"severity"`
	Status                     string `json:"status"`
	ExpectedAmountMinorUnits   int64  `json:"expected_amount_minor_units"`
	ActualAmountMinorUnits     int64  `json:"actual_amount_minor_units"`
	LeakageAmountMinorUnits    int64  `json:"leakage_amount_minor_units"`
	ConfidenceScoreBasisPoints uint16 `json:"confidence_score_basis_points"`
}

func (h *Handler) handleRunReconciliation(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var request runReconciliationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	command, err := request.toCommand()
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	result, err := h.reconciliation.RunRevenueLeakageCheck(r.Context(), command)
	if err != nil {
		writeError(w, stdhttp.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, newRunReconciliationResponse(result))
}

func (r runReconciliationRequest) toCommand() (appreconciliation.RunRevenueLeakageCheckCommand, error) {
	tenantID, err := parseUUID("tenant id", r.TenantID)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	contractID, err := parseUUID("contract id", r.ContractID)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	periodStart, err := parseTime("period start", r.Period.Start)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	periodEnd, err := parseTime("period end", r.Period.End)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	period, err := valueobject.NewBillingPeriod(periodStart, periodEnd)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, fmt.Errorf("build period: %w", err)
	}

	var minimumLeakageAmount valueobject.Money
	if r.Currency != "" || r.MinimumLeakageMinorUnits != 0 {
		minimumLeakageAmount, err = valueobject.NewMoney(r.Currency, r.MinimumLeakageMinorUnits)
		if err != nil {
			return appreconciliation.RunRevenueLeakageCheckCommand{}, fmt.Errorf(
				"build minimum leakage amount: %w",
				err,
			)
		}
	}

	return appreconciliation.RunRevenueLeakageCheckCommand{
		TenantID:             tenantID,
		ContractID:           contractID,
		PeriodStart:          period.Start,
		PeriodEnd:            period.End,
		TraceID:              r.TraceID,
		Currency:             r.Currency,
		MinimumLeakageAmount: minimumLeakageAmount,
	}, nil
}

func newRunReconciliationResponse(
	result appreconciliation.RunRevenueLeakageCheckResult,
) runReconciliationResponse {
	cases := make([]runReconciliationCase, 0, len(result.ReconciliationResult.Cases))
	for _, item := range result.ReconciliationResult.Cases {
		cases = append(cases, runReconciliationCase{
			ID:                         item.ID.String(),
			Type:                       string(item.Type),
			Severity:                   string(item.Severity),
			Status:                     string(item.Status),
			ExpectedAmountMinorUnits:   item.ExpectedAmount.MinorUnits,
			ActualAmountMinorUnits:     item.ActualAmount.MinorUnits,
			LeakageAmountMinorUnits:    item.LeakageAmount.MinorUnits,
			ConfidenceScoreBasisPoints: item.ConfidenceScore.BasisPoints,
		})
	}

	return runReconciliationResponse{
		RunID:                   result.ReconciliationResult.RunID.String(),
		ExpectedEntryID:         result.ExpectedEntry.String(),
		ActualEntryCount:        result.ActualEntryCount,
		DiffCount:               result.ReconciliationResult.DiffCount,
		CaseCount:               result.ReconciliationResult.CaseCount,
		LeakageAmountMinorUnits: result.ReconciliationResult.LeakageAmount.MinorUnits,
		Currency:                result.ReconciliationResult.LeakageAmount.Currency,
		Cases:                   cases,
	}
}

func parseUUID(field, raw string) (uuid.UUID, error) {
	value, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse %s: %w", field, err)
	}

	return value, nil
}

func parseTime(field, raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", field, err)
	}

	return value.UTC(), nil
}
