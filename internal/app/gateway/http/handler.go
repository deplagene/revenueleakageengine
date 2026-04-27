// Package http contains HTTP delivery handlers and DTO mapping for the gateway.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/go-chi/chi/v5"
)

// ErrReconciliationRunnerRequired reports that HTTP routes were created
// without a reconciliation application dependency.
var ErrReconciliationRunnerRequired = errors.New("reconciliation runner is required")

// ErrCaseQueriesRequired reports that HTTP routes were created without a case
// query dependency.
var ErrCaseQueriesRequired = errors.New("case queries are required")

// ErrCaseCommandsRequired reports that HTTP routes were created without a case
// command dependency.
var ErrCaseCommandsRequired = errors.New("case commands are required")

// ErrContractQueriesRequired reports that HTTP routes were created without a contract
// query dependency.
var ErrContractQueriesRequired = errors.New("contract queries are required")

// ErrContractCommandsRequired reports that HTTP routes were created without a contract
// command dependency.
var ErrContractCommandsRequired = errors.New("contract commands are required")

type reconciliationRunner interface {
	RunRevenueLeakageCheck(
		ctx context.Context,
		cmd appreconciliation.RunRevenueLeakageCheckCommand,
	) (appreconciliation.RunRevenueLeakageCheckResult, error)
}

type caseQueries interface {
	ListCases(
		ctx context.Context,
		cmd caseapp.ListCasesCommand,
	) (caseapp.ListCasesResult, error)
	GetCase(
		ctx context.Context,
		cmd caseapp.GetCaseCommand,
	) (caseapp.GetCaseResult, error)
}

type caseCommands interface {
	UpdateCaseStatus(
		ctx context.Context,
		cmd caseapp.UpdateCaseStatusCommand,
	) (caseapp.UpdateCaseStatusResult, error)
	UpdateCaseAssignee(
		ctx context.Context,
		cmd caseapp.UpdateCaseAssigneeCommand,
	) (caseapp.UpdateCaseAssigneeResult, error)
}

// Handler registers HTTP routes backed by application use cases.
type Handler struct {
	reconciliation   reconciliationRunner
	cases            caseQueries
	caseCommands     caseCommands
	contractQueries  contractQueries
	contractCommands contractCommands
}

// NewHandler constructs the gateway HTTP handler set.
func NewHandler(
	reconciliation reconciliationRunner,
	cases caseQueries,
	caseCommands caseCommands,
	contractQueries contractQueries,
	contractCommands contractCommands,
) (*Handler, error) {
	if reconciliation == nil {
		return nil, ErrReconciliationRunnerRequired
	}

	if cases == nil {
		return nil, ErrCaseQueriesRequired
	}

	if caseCommands == nil {
		return nil, ErrCaseCommandsRequired
	}

	if contractQueries == nil {
		return nil, ErrContractQueriesRequired
	}

	if contractCommands == nil {
		return nil, ErrContractCommandsRequired
	}

	return &Handler{
		reconciliation:   reconciliation,
		cases:            cases,
		caseCommands:     caseCommands,
		contractQueries:  contractQueries,
		contractCommands: contractCommands,
	}, nil
}

// RegisterRoutes attaches API routes to the provided router.
func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/api/v1", func(router chi.Router) {
		router.Post("/reconciliation/run", h.handleRunReconciliation)
		router.Get("/cases", h.handleListCases)
		router.Get("/cases/{case_id}", h.handleGetCase)
		router.Patch("/cases/{case_id}/status", h.handlePatchCaseStatus)
		router.Patch("/cases/{case_id}/resolve", h.handlePatchCaseResolve)
		router.Patch("/cases/{case_id}/dismiss", h.handlePatchCaseDismiss)
		router.Patch("/cases/{case_id}/assignee", h.handlePatchCaseAssignee)

		// Contracts
		router.Post("/contracts", h.handleUpsertContract)
		router.Get("/contracts/{contract_id}", h.handleGetContract)
		router.Get("/contracts/{contract_id}/terms/effective", h.handleGetEffectiveTerms)
		router.Post("/contracts/billable-items", h.handleUpsertBillableItem)
		router.Post("/contracts/terms", h.handleUpsertTerm)
	})
}

type errorResponse struct {
	Error string `json:"error"`
}

func decodeJSON(r *stdhttp.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode json body: %w", err)
	}

	return nil
}

func writeJSON(w stdhttp.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		stdhttp.Error(w, `{"error":"failed to encode response"}`, stdhttp.StatusInternalServerError)
	}
}

func writeError(w stdhttp.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, errorResponse{
		Error: message,
	})
}
