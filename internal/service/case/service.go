package casework

import (
	"context"
	"errors"
	"fmt"
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
