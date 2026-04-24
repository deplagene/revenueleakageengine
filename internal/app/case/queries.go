// Package caseapp contains application-level query entrypoints for leakage
// cases.
package caseapp

import (
	"context"
	"errors"

	casework "github.com/deplagene/revenueleakageengine/internal/service/case"
)

// ErrQueriesServiceRequired reports that case queries were created without
// a case service dependency.
var ErrQueriesServiceRequired = errors.New("case service is required")

// ErrCaseNotFound aliases the service-level sentinel used by case detail
// queries.
var ErrCaseNotFound = casework.ErrCaseNotFound

// ListCasesCommand aliases the case service query command at the application
// boundary.
type ListCasesCommand = casework.ListCasesCommand

// GetCaseCommand aliases the case service detail command at the application
// boundary.
type GetCaseCommand = casework.GetCaseCommand

// ListCasesResult aliases the case service query result at the application
// boundary.
type ListCasesResult = casework.ListCasesResult

// GetCaseResult aliases the case service detail result at the application
// boundary.
type GetCaseResult = casework.GetCaseResult

// Queries exposes case read-model use cases to transport layers.
type Queries struct {
	service *casework.Service
}

// NewQueries creates application-level case queries over a case service.
func NewQueries(service *casework.Service) (*Queries, error) {
	if service == nil {
		return nil, ErrQueriesServiceRequired
	}

	return &Queries{
		service: service,
	}, nil
}

// ListCases loads one page of leakage cases for the requested tenant.
func (q *Queries) ListCases(ctx context.Context, cmd ListCasesCommand) (ListCasesResult, error) {
	return q.service.ListCases(ctx, cmd)
}

// GetCase loads one detailed case for the requested tenant and case id.
func (q *Queries) GetCase(ctx context.Context, cmd GetCaseCommand) (GetCaseResult, error) {
	return q.service.GetCase(ctx, cmd)
}
