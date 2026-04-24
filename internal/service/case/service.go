package casework

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/google/uuid"
)

// ErrStoreRequired reports that a case service was created without a backing
// store.
var ErrStoreRequired = errors.New("case store is required")

// Service coordinates leakage case read-model operations.
type Service struct {
	store Store
}

// NewService creates a case service over the provided store.
func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, ErrStoreRequired
	}

	return &Service{
		store: store,
	}, nil
}

// ListCases loads one page of cases from the backing read model.
func (s *Service) ListCases(ctx context.Context, cmd ListCasesCommand) (ListCasesResult, error) {
	const op = "service.case.ListCases"

	cmd = cmd.Normalize()
	if err := cmd.Validate(); err != nil {
		return ListCasesResult{}, fmt.Errorf("%s: %w", op, err)
	}

	cases, err := s.store.ListCases(ctx, cmd)
	if err != nil {
		return ListCasesResult{}, fmt.Errorf("%s: list cases: %w", op, err)
	}

	return ListCasesResult{
		Cases: cases,
	}, nil
}

// GetCase loads one detailed case view from the backing read model.
func (s *Service) GetCase(ctx context.Context, cmd GetCaseCommand) (GetCaseResult, error) {
	const op = "service.case.GetCase"

	if err := cmd.Validate(); err != nil {
		return GetCaseResult{}, fmt.Errorf("%s: %w", op, err)
	}

	result, err := s.store.GetCase(ctx, cmd)
	if err != nil {
		return GetCaseResult{}, fmt.Errorf("%s: get case: %w", op, err)
	}

	return result, nil
}

// UpdateCaseStatus applies one lifecycle transition to a leakage case and
// persists a status history audit record.
func (s *Service) UpdateCaseStatus(
	ctx context.Context,
	cmd UpdateCaseStatusCommand,
) (UpdateCaseStatusResult, error) {
	const op = "service.case.UpdateCaseStatus"

	if err := cmd.Validate(); err != nil {
		return UpdateCaseStatusResult{}, fmt.Errorf("%s: %w", op, err)
	}

	current, err := s.store.GetCase(ctx, GetCaseCommand{
		TenantID: cmd.TenantID,
		CaseID:   cmd.CaseID,
	})
	if err != nil {
		return UpdateCaseStatusResult{}, fmt.Errorf("%s: get current case: %w", op, err)
	}

	if current.Case.Status == cmd.Status {
		return UpdateCaseStatusResult{
			Case: current.Case,
		}, nil
	}

	if !current.Case.Status.CanTransitionTo(cmd.Status) {
		return UpdateCaseStatusResult{}, fmt.Errorf("%s: %w", op, leakage.ErrInvalidStatusTransition)
	}

	updatedCase := current.Case
	updatedCase.Status = cmd.Status

	history := leakage.StatusHistory{
		ID:         uuid.New(),
		CaseID:     updatedCase.ID,
		FromStatus: current.Case.Status,
		ToStatus:   cmd.Status,
		ChangedAt:  time.Now().UTC(),
		ChangedBy:  cmd.ChangedBy,
	}

	if err := s.store.UpdateCaseStatus(ctx, updatedCase, history); err != nil {
		return UpdateCaseStatusResult{}, fmt.Errorf("%s: update case status: %w", op, err)
	}

	return UpdateCaseStatusResult{
		Case: updatedCase,
	}, nil
}

// UpdateCaseAssignee applies one ownership change to a leakage case.
func (s *Service) UpdateCaseAssignee(
	ctx context.Context,
	cmd UpdateCaseAssigneeCommand,
) (UpdateCaseAssigneeResult, error) {
	const op = "service.case.UpdateCaseAssignee"

	if err := cmd.Validate(); err != nil {
		return UpdateCaseAssigneeResult{}, fmt.Errorf("%s: %w", op, err)
	}

	assignee := strings.TrimSpace(cmd.Assignee)
	if assignee == "" {
		return UpdateCaseAssigneeResult{}, fmt.Errorf("%s: %w", op, ErrAssigneeRequired)
	}

	current, err := s.store.GetCase(ctx, GetCaseCommand{
		TenantID: cmd.TenantID,
		CaseID:   cmd.CaseID,
	})
	if err != nil {
		return UpdateCaseAssigneeResult{}, fmt.Errorf("%s: get current case: %w", op, err)
	}

	if current.Case.Assignee == assignee {
		return UpdateCaseAssigneeResult{
			Case: current.Case,
		}, nil
	}

	updatedCase := current.Case
	updatedCase.Assignee = assignee

	if err := s.store.UpdateCaseAssignee(ctx, updatedCase); err != nil {
		return UpdateCaseAssigneeResult{}, fmt.Errorf("%s: update case assignee: %w", op, err)
	}

	return UpdateCaseAssigneeResult{
		Case: updatedCase,
	}, nil
}
