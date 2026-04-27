package http

import (
	"context"
	"net/http"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type contractQueries interface {
	GetContract(ctx context.Context, id uuid.UUID) (*contract.Contract, error)
	ListContractsByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]*contract.Contract, error)
	GetEffectiveTerms(ctx context.Context, contractID uuid.UUID, at time.Time) ([]*contract.Term, error)
}

type contractCommands interface {
	UpsertContract(ctx context.Context, con *contract.Contract) error
	UpsertBillableItem(ctx context.Context, item *contract.BillableItem) error
	UpsertTerm(ctx context.Context, term *contract.Term) error
}

func (h *Handler) handleUpsertContract(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID           string         `json:"id"`
		TenantID     string         `json:"tenant_id"`
		CustomerID   string         `json:"customer_id"`
		ExternalID   string         `json:"external_id"`
		Status       string         `json:"status"`
		StartDate    time.Time      `json:"start_date"`
		EndDate      *time.Time     `json:"end_date"`
		Currency     string         `json:"currency"`
		Version      int            `json:"version"`
		BillingModel string         `json:"billing_model"`
		SignedAt     *time.Time     `json:"signed_at"`
		Metadata     map[string]any `json:"metadata"`
	}

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, _ := uuid.Parse(req.ID)
	tenantID, _ := uuid.Parse(req.TenantID)
	customerID, _ := uuid.Parse(req.CustomerID)

	con := &contract.Contract{
		ID:           id,
		TenantID:     tenantID,
		CustomerID:   customerID,
		ExternalID:   req.ExternalID,
		Status:       contract.Status(req.Status),
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Currency:     req.Currency,
		Version:      req.Version,
		BillingModel: contract.BillingModel(req.BillingModel),
		SignedAt:     req.SignedAt,
		Metadata:     req.Metadata,
	}

	if err := h.contractCommands.UpsertContract(r.Context(), con); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, con)
}

func (h *Handler) handleGetContract(w http.ResponseWriter, r *http.Request) {
	contractIDStr := chi.URLParam(r, "contract_id")
	contractID, err := uuid.Parse(contractIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid contract_id")
		return
	}

	con, err := h.contractQueries.GetContract(r.Context(), contractID)
	if err != nil {
		writeError(w, http.StatusNotFound, "contract not found")
		return
	}

	writeJSON(w, http.StatusOK, con)
}

func (h *Handler) handleUpsertBillableItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          string `json:"id"`
		TenantID    string `json:"tenant_id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		Category    string `json:"category"`
		Unit        string `json:"unit"`
		PricingMode string `json:"pricing_mode"`
		Status      string `json:"status"`
	}

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, _ := uuid.Parse(req.ID)
	tenantID, _ := uuid.Parse(req.TenantID)

	item := &contract.BillableItem{
		ID:          id,
		TenantID:    tenantID,
		Code:        req.Code,
		Name:        req.Name,
		Category:    req.Category,
		Unit:        req.Unit,
		PricingMode: contract.PricingMode(req.PricingMode),
		Status:      contract.BillableItemStatus(req.Status),
	}

	if err := h.contractCommands.UpsertBillableItem(r.Context(), item); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) handleUpsertTerm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID            string         `json:"id"`
		TenantID      string         `json:"tenant_id"`
		ContractID    string         `json:"contract_id"`
		Type          string         `json:"type"`
		EffectiveFrom time.Time      `json:"effective_from"`
		EffectiveTo   *time.Time     `json:"effective_to"`
		Priority      int            `json:"priority"`
		Expression    map[string]any `json:"expression"`
		SourceRef     string         `json:"source_ref"`
	}

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, _ := uuid.Parse(req.ID)
	tenantID, _ := uuid.Parse(req.TenantID)
	contractID, _ := uuid.Parse(req.ContractID)

	term := &contract.Term{
		ID:            id,
		TenantID:      tenantID,
		ContractID:    contractID,
		Type:          contract.TermType(req.Type),
		EffectiveFrom: req.EffectiveFrom,
		EffectiveTo:   req.EffectiveTo,
		Priority:      req.Priority,
		Expression:    req.Expression,
		SourceRef:     req.SourceRef,
	}

	if err := h.contractCommands.UpsertTerm(r.Context(), term); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, term)
}

func (h *Handler) handleGetEffectiveTerms(w http.ResponseWriter, r *http.Request) {
	contractIDStr := chi.URLParam(r, "contract_id")
	contractID, err := uuid.Parse(contractIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid contract_id")
		return
	}

	atStr := r.URL.Query().Get("at")
	at := time.Now()
	if atStr != "" {
		if parsedAt, err := time.Parse(time.RFC3339, atStr); err == nil {
			at = parsedAt
		}
	}

	terms, err := h.contractQueries.GetEffectiveTerms(r.Context(), contractID, at)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, terms)
}
