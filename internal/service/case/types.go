// Package casework contains business operations for leakage case lifecycle and
// investigation workflow.
package casework

import (
	"errors"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/google/uuid"
)

const (
	defaultListLimit = 50
	maxListLimit     = 100
)

var (
	// ErrTenantRequired reports that a case query was requested without tenant
	// scope.
	ErrTenantRequired = errors.New("tenant id is required")
	// ErrCaseIDRequired reports that a case query was requested without a case
	// id.
	ErrCaseIDRequired = errors.New("case id is required")
	// ErrCaseNotFound reports that the requested case does not exist in the
	// requested tenant scope.
	ErrCaseNotFound = errors.New("case not found")
	// ErrLimitInvalid reports that the requested page size is not positive.
	ErrLimitInvalid = errors.New("limit must be greater than zero")
	// ErrOffsetInvalid reports that the requested page offset is negative.
	ErrOffsetInvalid = errors.New("offset cannot be negative")
)

// ListCasesCommand describes one read-model query for leakage cases.
type ListCasesCommand struct {
	TenantID   uuid.UUID
	ContractID uuid.UUID
	Status     leakage.Status
	Limit      int
	Offset     int
}

// Normalize applies safe defaults and upper bounds to paging parameters.
func (c ListCasesCommand) Normalize() ListCasesCommand {
	if c.Limit == 0 {
		c.Limit = defaultListLimit
	}

	if c.Limit > maxListLimit {
		c.Limit = maxListLimit
	}

	return c
}

// Validate checks that the case query can be executed safely.
func (c ListCasesCommand) Validate() error {
	if c.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if c.Limit < 0 {
		return ErrLimitInvalid
	}

	if c.Offset < 0 {
		return ErrOffsetInvalid
	}

	return nil
}

// ListCasesResult contains one page of leakage cases.
type ListCasesResult struct {
	Cases []leakage.Case
}

// GetCaseCommand describes one detailed case lookup query.
type GetCaseCommand struct {
	TenantID uuid.UUID
	CaseID   uuid.UUID
}

// Validate checks that the detailed case query has enough scope to execute.
func (c GetCaseCommand) Validate() error {
	if c.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if c.CaseID == uuid.Nil {
		return ErrCaseIDRequired
	}

	return nil
}

// GetCaseResult contains the primary case together with its attached evidence
// and root-cause hypotheses.
type GetCaseResult struct {
	Case       leakage.Case
	Evidence   []leakage.Evidence
	RootCauses []leakage.RootCause
}
