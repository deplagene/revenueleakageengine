package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	contractdomain "github.com/deplagene/revenueleakageengine/internal/domain/contract"
	contractservice "github.com/deplagene/revenueleakageengine/internal/service/contract"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type contractQueries interface {
	GetContract(ctx context.Context, id uuid.UUID) (*contractdomain.Contract, error)
	ListContractsByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]*contractdomain.Contract, error)
	GetEffectiveTerms(ctx context.Context, contractID uuid.UUID, at time.Time) ([]*contractdomain.Term, error)
}

type contractCommands interface {
	UpsertContract(ctx context.Context, con *contractdomain.Contract) error
	UpsertBillableItem(ctx context.Context, item *contractdomain.BillableItem) error
	UpsertTerm(ctx context.Context, term *contractdomain.Term) error
}

var contractValidationErrors = []error{
	contractservice.ErrContractRequired,
	contractservice.ErrBillableItemRequired,
	contractservice.ErrTermRequired,
	contractdomain.ErrContractIDRequired,
	contractdomain.ErrTenantRequired,
	contractdomain.ErrCustomerRequired,
	contractdomain.ErrStatusRequired,
	contractdomain.ErrStatusInvalid,
	contractdomain.ErrStartDateRequired,
	contractdomain.ErrContractDateRangeInvalid,
	contractdomain.ErrCurrencyRequired,
	contractdomain.ErrVersionInvalid,
	contractdomain.ErrBillingModelRequired,
	contractdomain.ErrBillingModelInvalid,
	contractdomain.ErrBillableItemIDRequired,
	contractdomain.ErrBillableItemCodeRequired,
	contractdomain.ErrBillableItemNameRequired,
	contractdomain.ErrBillableItemUnitRequired,
	contractdomain.ErrPricingModeRequired,
	contractdomain.ErrPricingModeInvalid,
	contractdomain.ErrBillableItemStatusRequired,
	contractdomain.ErrBillableItemStatusInvalid,
	contractdomain.ErrTermIDRequired,
	contractdomain.ErrTermTypeRequired,
	contractdomain.ErrTermTypeInvalid,
	contractdomain.ErrEffectiveFromRequired,
	contractdomain.ErrEffectiveRangeInvalid,
	contractdomain.ErrTermExpressionRequired,
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

	id, err := optionalUUID("id", req.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, err := requiredUUID("tenant_id", req.TenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	customerID, err := requiredUUID("customer_id", req.CustomerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	con := &contractdomain.Contract{
		ID:           id,
		TenantID:     tenantID,
		CustomerID:   customerID,
		ExternalID:   req.ExternalID,
		Status:       contractdomain.Status(req.Status),
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Currency:     req.Currency,
		Version:      req.Version,
		BillingModel: contractdomain.BillingModel(req.BillingModel),
		SignedAt:     req.SignedAt,
		Metadata:     req.Metadata,
	}

	if err := h.contractCommands.UpsertContract(r.Context(), con); err != nil {
		writeContractError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, con)
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
		writeContractError(w, err)
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

	id, err := optionalUUID("id", req.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, err := requiredUUID("tenant_id", req.TenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	item := &contractdomain.BillableItem{
		ID:          id,
		TenantID:    tenantID,
		Code:        req.Code,
		Name:        req.Name,
		Category:    req.Category,
		Unit:        req.Unit,
		PricingMode: contractdomain.PricingMode(req.PricingMode),
		Status:      contractdomain.BillableItemStatus(req.Status),
	}

	if err := h.contractCommands.UpsertBillableItem(r.Context(), item); err != nil {
		writeContractError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) handleUpsertTerm(w http.ResponseWriter, r *http.Request) {
	contractID, ok := contractIDFromRoute(w, r)
	if !ok {
		return
	}

	h.upsertTerm(w, r, contractID)
}

func (h *Handler) handleUpsertTermLegacy(w http.ResponseWriter, r *http.Request) {
	h.upsertTerm(w, r, uuid.Nil)
}

func (h *Handler) upsertTerm(w http.ResponseWriter, r *http.Request, routeContractID uuid.UUID) {
	var req struct {
		ID            string         `json:"id"`
		TenantID      string         `json:"tenant_id"`
		ContractID    string         `json:"contract_id"`
		Type          string         `json:"type"`
		Code          string         `json:"code"`
		Amount        *int64         `json:"amount"`
		Currency      string         `json:"currency"`
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

	id, err := optionalUUID("id", req.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, err := optionalUUID("tenant_id", req.TenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	bodyContractID, err := optionalUUID("contract_id", req.ContractID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	contractID, err := resolveTermContractID(routeContractID, bodyContractID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	expression := termExpressionFromRequest(
		req.Expression,
		req.Code,
		req.Amount,
		req.Currency,
	)

	term := &contractdomain.Term{
		ID:            id,
		TenantID:      tenantID,
		ContractID:    contractID,
		Type:          contractdomain.TermType(req.Type),
		EffectiveFrom: req.EffectiveFrom,
		EffectiveTo:   req.EffectiveTo,
		Priority:      req.Priority,
		Expression:    expression,
		SourceRef:     req.SourceRef,
	}

	if err := h.contractCommands.UpsertTerm(r.Context(), term); err != nil {
		writeContractError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, term)
}

func (h *Handler) handleGetEffectiveTerms(w http.ResponseWriter, r *http.Request) {
	contractIDStr := chi.URLParam(r, "contract_id")
	contractID, err := uuid.Parse(contractIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid contract_id")
		return
	}

	atStr := r.URL.Query().Get("at")
	at := time.Time{}
	if atStr != "" {
		parsedAt, err := time.Parse(time.RFC3339, atStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid at timestamp")
			return
		}

		at = parsedAt
	}

	terms, err := h.contractQueries.GetEffectiveTerms(r.Context(), contractID, at)
	if err != nil {
		writeContractError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, terms)
}

func contractIDFromRoute(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	contractID, err := requiredUUID("contract_id", chi.URLParam(r, "contract_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return uuid.Nil, false
	}

	return contractID, true
}

func requiredUUID(field, value string) (uuid.UUID, error) {
	id, err := parseUUID(field, value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s", field)
	}

	return id, nil
}

func resolveTermContractID(routeContractID, bodyContractID uuid.UUID) (uuid.UUID, error) {
	if routeContractID != uuid.Nil {
		if bodyContractID != uuid.Nil && bodyContractID != routeContractID {
			return uuid.Nil, errors.New("contract_id does not match route")
		}

		return routeContractID, nil
	}

	if bodyContractID == uuid.Nil {
		return uuid.Nil, errors.New("contract_id is required")
	}

	return bodyContractID, nil
}

func termExpressionFromRequest(
	expression map[string]any,
	code string,
	amount *int64,
	currency string,
) map[string]any {
	if expression == nil {
		expression = map[string]any{}
	}

	if code != "" {
		expression["code"] = code
	}

	if amount != nil {
		expression["amount_minor_units"] = *amount
	}

	if currency != "" {
		expression["currency"] = currency
	}

	return expression
}

func writeContractError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, contractservice.ErrContractNotFound):
		writeError(w, http.StatusNotFound, contractservice.ErrContractNotFound.Error())
	case isContractValidationError(err):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func isContractValidationError(err error) bool {
	for _, target := range contractValidationErrors {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}
