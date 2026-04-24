package casework

import (
	"context"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
)

// Store defines the read and write dependencies required by case workflows.
type Store interface {
	// ListCases returns one page of leakage cases for the requested tenant and
	// optional filters.
	ListCases(ctx context.Context, cmd ListCasesCommand) ([]leakage.Case, error)
	// GetCase returns one leakage case together with its evidence and root
	// causes.
	GetCase(ctx context.Context, cmd GetCaseCommand) (GetCaseResult, error)
	// UpdateCaseStatus persists one case status transition and its audit record.
	UpdateCaseStatus(ctx context.Context, c leakage.Case, history leakage.StatusHistory) error
	// UpdateCaseAssignee persists one case ownership update.
	UpdateCaseAssignee(ctx context.Context, c leakage.Case) error
}
