package http

import (
	"errors"
	"fmt"
	stdhttp "net/http"
	"strconv"
	"time"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const defaultCasesLimit = 50

type listCasesResponse struct {
	Cases []caseResponse `json:"cases"`
}

type getCaseResponse struct {
	Case       caseResponse                `json:"case"`
	Evidence   []caseEvidenceResponse      `json:"evidence"`
	RootCauses []rootCauseResponse         `json:"root_causes"`
	History    []caseStatusHistoryResponse `json:"status_history"`
}

type caseResponse struct {
	ID                         string `json:"id"`
	TenantID                   string `json:"tenant_id"`
	CustomerID                 string `json:"customer_id"`
	ContractID                 string `json:"contract_id"`
	Type                       string `json:"type"`
	Severity                   string `json:"severity"`
	Status                     string `json:"status"`
	DetectedAt                 string `json:"detected_at"`
	PeriodStart                string `json:"period_start"`
	PeriodEnd                  string `json:"period_end"`
	ExpectedAmountMinorUnits   int64  `json:"expected_amount_minor_units"`
	ActualAmountMinorUnits     int64  `json:"actual_amount_minor_units"`
	LeakageAmountMinorUnits    int64  `json:"leakage_amount_minor_units"`
	Currency                   string `json:"currency"`
	ConfidenceScoreBasisPoints uint16 `json:"confidence_score_basis_points"`
	RootCauseCategory          string `json:"root_cause_category"`
	Assignee                   string `json:"assignee"`
	TraceID                    string `json:"trace_id"`
}

type caseEvidenceResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  string         `json:"created_at"`
}

type rootCauseResponse struct {
	ID                         string `json:"id"`
	Category                   string `json:"category"`
	Subcategory                string `json:"subcategory"`
	Description                string `json:"description"`
	ConfidenceScoreBasisPoints uint16 `json:"confidence_score_basis_points"`
	DerivedBy                  string `json:"derived_by"`
	CreatedAt                  string `json:"created_at"`
}

type caseStatusHistoryResponse struct {
	ID         string `json:"id"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	ChangedAt  string `json:"changed_at"`
	ChangedBy  string `json:"changed_by"`
	ReasonCode string `json:"reason_code"`
	Comment    string `json:"comment"`
}

type patchCaseStatusRequest struct {
	TenantID  string `json:"tenant_id"`
	Status    string `json:"status"`
	ChangedBy string `json:"changed_by"`
}

type patchCaseStatusResponse struct {
	Case caseResponse `json:"case"`
}

type patchCaseDecisionRequest struct {
	TenantID   string `json:"tenant_id"`
	ReasonCode string `json:"reason_code"`
	Comment    string `json:"comment"`
	ChangedBy  string `json:"changed_by"`
}

type patchCaseDecisionResponse struct {
	Case caseResponse `json:"case"`
}

type patchCaseAssigneeRequest struct {
	TenantID string `json:"tenant_id"`
	Assignee string `json:"assignee"`
}

type patchCaseAssigneeResponse struct {
	Case caseResponse `json:"case"`
}

func (h *Handler) handleListCases(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	command, err := listCasesCommandFromRequest(r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	result, err := h.cases.ListCases(r.Context(), command)
	if err != nil {
		writeError(w, stdhttp.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, newListCasesResponse(result))
}

func (h *Handler) handleGetCase(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	command, err := getCaseCommandFromRequest(r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	result, err := h.cases.GetCase(r.Context(), command)
	if err != nil {
		statusCode := stdhttp.StatusUnprocessableEntity
		if errors.Is(err, caseapp.ErrCaseNotFound) {
			statusCode = stdhttp.StatusNotFound
		}

		writeError(w, statusCode, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, newGetCaseResponse(result))
}

func (h *Handler) handlePatchCaseStatus(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var request patchCaseStatusRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	command, err := patchCaseStatusCommandFromRequest(r, request)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	result, err := h.caseCommands.UpdateCaseStatus(r.Context(), command)
	if err != nil {
		statusCode := stdhttp.StatusUnprocessableEntity
		if errors.Is(err, caseapp.ErrCaseNotFound) {
			statusCode = stdhttp.StatusNotFound
		}

		writeError(w, statusCode, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, patchCaseStatusResponse{
		Case: newCaseResponse(result.Case),
	})
}

func (h *Handler) handlePatchCaseResolve(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	h.handlePatchCaseDecision(w, r, leakage.StatusResolved)
}

func (h *Handler) handlePatchCaseDismiss(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	h.handlePatchCaseDecision(w, r, leakage.StatusDismissed)
}

func (h *Handler) handlePatchCaseAssignee(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var request patchCaseAssigneeRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	command, err := patchCaseAssigneeCommandFromRequest(r, request)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	result, err := h.caseCommands.UpdateCaseAssignee(r.Context(), command)
	if err != nil {
		statusCode := stdhttp.StatusUnprocessableEntity
		if errors.Is(err, caseapp.ErrCaseNotFound) {
			statusCode = stdhttp.StatusNotFound
		}

		writeError(w, statusCode, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, patchCaseAssigneeResponse{
		Case: newCaseResponse(result.Case),
	})
}

func (h *Handler) handlePatchCaseDecision(
	w stdhttp.ResponseWriter,
	r *stdhttp.Request,
	targetStatus leakage.Status,
) {
	var request patchCaseDecisionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	command, err := patchCaseDecisionCommandFromRequest(r, request, targetStatus)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	result, err := h.caseCommands.UpdateCaseStatus(r.Context(), command)
	if err != nil {
		statusCode := stdhttp.StatusUnprocessableEntity
		if errors.Is(err, caseapp.ErrCaseNotFound) {
			statusCode = stdhttp.StatusNotFound
		}

		writeError(w, statusCode, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, patchCaseDecisionResponse{
		Case: newCaseResponse(result.Case),
	})
}

func listCasesCommandFromRequest(r *stdhttp.Request) (caseapp.ListCasesCommand, error) {
	tenantID, err := parseUUID("tenant id", r.URL.Query().Get("tenant_id"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	contractID, err := optionalUUID("contract id", r.URL.Query().Get("contract_id"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	status, err := optionalCaseStatus(r.URL.Query().Get("status"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	limit, err := optionalIntQuery("limit", r.URL.Query().Get("limit"), defaultCasesLimit)
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	offset, err := optionalIntQuery("offset", r.URL.Query().Get("offset"), 0)
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	return caseapp.ListCasesCommand{
		TenantID:   tenantID,
		ContractID: contractID,
		Status:     status,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

func newListCasesResponse(result caseapp.ListCasesResult) listCasesResponse {
	cases := make([]caseResponse, 0, len(result.Cases))
	for _, item := range result.Cases {
		cases = append(cases, newCaseResponse(item))
	}

	return listCasesResponse{
		Cases: cases,
	}
}

func getCaseCommandFromRequest(r *stdhttp.Request) (caseapp.GetCaseCommand, error) {
	tenantID, err := parseUUID("tenant id", r.URL.Query().Get("tenant_id"))
	if err != nil {
		return caseapp.GetCaseCommand{}, err
	}

	caseID, err := parseUUID("case id", chi.URLParam(r, "case_id"))
	if err != nil {
		return caseapp.GetCaseCommand{}, err
	}

	return caseapp.GetCaseCommand{
		TenantID: tenantID,
		CaseID:   caseID,
	}, nil
}

func patchCaseStatusCommandFromRequest(
	r *stdhttp.Request,
	request patchCaseStatusRequest,
) (caseapp.UpdateCaseStatusCommand, error) {
	tenantID, err := parseUUID("tenant id", request.TenantID)
	if err != nil {
		return caseapp.UpdateCaseStatusCommand{}, err
	}

	caseID, err := parseUUID("case id", chi.URLParam(r, "case_id"))
	if err != nil {
		return caseapp.UpdateCaseStatusCommand{}, err
	}

	status, err := parseOperationalCaseStatus(request.Status)
	if err != nil {
		return caseapp.UpdateCaseStatusCommand{}, err
	}

	return caseapp.UpdateCaseStatusCommand{
		TenantID:   tenantID,
		CaseID:     caseID,
		Status:     status,
		ChangedBy:  request.ChangedBy,
		ReasonCode: "",
		Comment:    "",
	}, nil
}

func patchCaseDecisionCommandFromRequest(
	r *stdhttp.Request,
	request patchCaseDecisionRequest,
	targetStatus leakage.Status,
) (caseapp.UpdateCaseStatusCommand, error) {
	tenantID, err := parseUUID("tenant id", request.TenantID)
	if err != nil {
		return caseapp.UpdateCaseStatusCommand{}, err
	}

	caseID, err := parseUUID("case id", chi.URLParam(r, "case_id"))
	if err != nil {
		return caseapp.UpdateCaseStatusCommand{}, err
	}

	return caseapp.UpdateCaseStatusCommand{
		TenantID:   tenantID,
		CaseID:     caseID,
		Status:     targetStatus,
		ChangedBy:  request.ChangedBy,
		ReasonCode: request.ReasonCode,
		Comment:    request.Comment,
	}, nil
}

func patchCaseAssigneeCommandFromRequest(
	r *stdhttp.Request,
	request patchCaseAssigneeRequest,
) (caseapp.UpdateCaseAssigneeCommand, error) {
	tenantID, err := parseUUID("tenant id", request.TenantID)
	if err != nil {
		return caseapp.UpdateCaseAssigneeCommand{}, err
	}

	caseID, err := parseUUID("case id", chi.URLParam(r, "case_id"))
	if err != nil {
		return caseapp.UpdateCaseAssigneeCommand{}, err
	}

	return caseapp.UpdateCaseAssigneeCommand{
		TenantID: tenantID,
		CaseID:   caseID,
		Assignee: request.Assignee,
	}, nil
}

func newGetCaseResponse(result caseapp.GetCaseResult) getCaseResponse {
	evidence := make([]caseEvidenceResponse, 0, len(result.Evidence))
	for _, item := range result.Evidence {
		evidence = append(evidence, caseEvidenceResponse{
			ID:         item.ID.String(),
			Type:       string(item.Type),
			EntityType: item.EntityType,
			EntityID:   item.EntityID,
			Payload:    item.Payload,
			CreatedAt:  item.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	rootCauses := make([]rootCauseResponse, 0, len(result.RootCauses))
	for _, item := range result.RootCauses {
		rootCauses = append(rootCauses, rootCauseResponse{
			ID:                         item.ID.String(),
			Category:                   string(item.Category),
			Subcategory:                item.Subcategory,
			Description:                item.Description,
			ConfidenceScoreBasisPoints: item.ConfidenceScore.BasisPoints,
			DerivedBy:                  string(item.DerivedBy),
			CreatedAt:                  item.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	history := make([]caseStatusHistoryResponse, 0, len(result.History))
	for _, item := range result.History {
		history = append(history, caseStatusHistoryResponse{
			ID:         item.ID.String(),
			FromStatus: string(item.FromStatus),
			ToStatus:   string(item.ToStatus),
			ChangedAt:  item.ChangedAt.UTC().Format(time.RFC3339Nano),
			ChangedBy:  item.ChangedBy,
			ReasonCode: item.ReasonCode,
			Comment:    item.Comment,
		})
	}

	return getCaseResponse{
		Case:       newCaseResponse(result.Case),
		Evidence:   evidence,
		RootCauses: rootCauses,
		History:    history,
	}
}

func newCaseResponse(item leakage.Case) caseResponse {
	return caseResponse{
		ID:                         item.ID.String(),
		TenantID:                   item.TenantID.String(),
		CustomerID:                 item.CustomerID.String(),
		ContractID:                 item.ContractID.String(),
		Type:                       string(item.Type),
		Severity:                   string(item.Severity),
		Status:                     string(item.Status),
		DetectedAt:                 item.DetectedAt.UTC().Format(time.RFC3339Nano),
		PeriodStart:                item.Period.Start.UTC().Format(time.RFC3339Nano),
		PeriodEnd:                  item.Period.End.UTC().Format(time.RFC3339Nano),
		ExpectedAmountMinorUnits:   item.ExpectedAmount.MinorUnits,
		ActualAmountMinorUnits:     item.ActualAmount.MinorUnits,
		LeakageAmountMinorUnits:    item.LeakageAmount.MinorUnits,
		Currency:                   item.LeakageAmount.Currency,
		ConfidenceScoreBasisPoints: item.ConfidenceScore.BasisPoints,
		RootCauseCategory:          string(item.RootCauseCategory),
		Assignee:                   item.Assignee,
		TraceID:                    item.TraceID,
	}
}

func optionalUUID(field, raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.UUID{}, nil
	}

	value, err := parseUUID(field, raw)
	if err != nil {
		return uuid.UUID{}, err
	}

	return value, nil
}

func optionalCaseStatus(raw string) (leakage.Status, error) {
	if raw == "" {
		return "", nil
	}

	return parseCaseStatus(raw)
}

func parseCaseStatus(raw string) (leakage.Status, error) {
	switch leakage.Status(raw) {
	case leakage.StatusOpen:
		return leakage.StatusOpen, nil
	case leakage.StatusInvestigating:
		return leakage.StatusInvestigating, nil
	case leakage.StatusResolved:
		return leakage.StatusResolved, nil
	case leakage.StatusDismissed:
		return leakage.StatusDismissed, nil
	default:
		return "", fmt.Errorf("parse status: invalid status %q", raw)
	}
}

func parseOperationalCaseStatus(raw string) (leakage.Status, error) {
	switch leakage.Status(raw) {
	case leakage.StatusOpen:
		return leakage.StatusOpen, nil
	case leakage.StatusInvestigating:
		return leakage.StatusInvestigating, nil
	default:
		return "", fmt.Errorf("parse status: only open or investigating are supported on this endpoint")
	}
}

func optionalIntQuery(field, raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}

	return value, nil
}
