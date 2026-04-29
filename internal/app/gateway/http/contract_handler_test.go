package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	contractdomain "github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type recordingContractCommands struct {
	contract *contractdomain.Contract
	item     *contractdomain.BillableItem
	term     *contractdomain.Term
	err      error
}

func (c *recordingContractCommands) UpsertContract(
	_ context.Context,
	contract *contractdomain.Contract,
) error {
	c.contract = contract
	return c.err
}

func (c *recordingContractCommands) UpsertBillableItem(
	_ context.Context,
	item *contractdomain.BillableItem,
) error {
	c.item = item
	return c.err
}

func (c *recordingContractCommands) UpsertTerm(
	_ context.Context,
	term *contractdomain.Term,
) error {
	c.term = term
	return c.err
}

func TestHandlerUpsertTermUsesContractRouteAndSprintPayload(t *testing.T) {
	t.Parallel()

	contractID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	commands := &recordingContractCommands{}
	handler := newContractTestHandler(t, commands)

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/contracts/"+contractID.String()+"/terms",
		strings.NewReader(`{
			"type": "fixed_fee",
			"code": "platform_subscription",
			"amount": 50000,
			"currency": "USD",
			"effective_from": "2024-01-01T00:00:00Z"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusCreated {
		t.Fatalf("status code = %d, want %d, body=%s", response.Code, stdhttp.StatusCreated, response.Body.String())
	}

	if commands.term == nil {
		t.Fatal("contract term command was not received")
	}

	if commands.term.ContractID != contractID {
		t.Fatalf("contract id = %s, want %s", commands.term.ContractID, contractID)
	}

	if commands.term.Type != contractdomain.TermTypeFixedFee {
		t.Fatalf("term type = %s, want %s", commands.term.Type, contractdomain.TermTypeFixedFee)
	}

	if got := commands.term.Expression["code"]; got != "platform_subscription" {
		t.Fatalf("expression code = %v, want platform_subscription", got)
	}

	if got := commands.term.Expression["amount_minor_units"]; got != int64(50000) {
		t.Fatalf("expression amount = %v, want 50000", got)
	}

	if got := commands.term.Expression["currency"]; got != "USD" {
		t.Fatalf("expression currency = %v, want USD", got)
	}
}

func TestHandlerUpsertContractRejectsInvalidUUID(t *testing.T) {
	t.Parallel()

	handler := newContractTestHandler(t, &recordingContractCommands{})

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/contracts",
		strings.NewReader(`{
			"tenant_id": "bad-uuid",
			"customer_id": "22222222-2222-2222-2222-222222222222",
			"status": "active",
			"start_date": "2026-01-01T00:00:00Z",
			"currency": "USD",
			"version": 1,
			"billing_model": "fixed_recurring"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, stdhttp.StatusBadRequest)
	}
}

func TestHandlerGetEffectiveTermsRejectsInvalidAt(t *testing.T) {
	t.Parallel()

	contractID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	handler := newContractTestHandler(t, &recordingContractCommands{})

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/v1/contracts/"+contractID.String()+"/terms/effective?at=not-a-time",
		nil,
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, stdhttp.StatusBadRequest)
	}
}

func newContractTestHandler(t *testing.T, commands *recordingContractCommands) *Handler {
	t.Helper()

	handler, err := NewHandler(
		&noopReconciliationRunner{},
		&stubCaseQueries{},
		&stubCaseCommands{},
		&noopContractQueries{},
		commands,
		&noopIngestionCommands{},
	)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	return handler
}
