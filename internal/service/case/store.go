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
}
