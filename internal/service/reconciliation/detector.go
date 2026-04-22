package reconciliation

import (
	"context"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
)

// Detector identifies leakage candidates from calculated reconciliation diffs.
type Detector interface {
	Detect(ctx context.Context, cmd ReconcilePeriodCommand, diffs []Diff) ([]LeakageCandidate, error)
}

type defaultDetector struct{}

// NewDetector returns the default deterministic detector for MVP leakage rules.
func NewDetector() Detector {
	return defaultDetector{}
}

// Detect classifies positive revenue gaps as leakage candidates. The first MVP
// rules are deliberately simple: missing actual revenue becomes unbilled usage,
// and partial actual revenue becomes underbilling.
func (d defaultDetector) Detect(
	ctx context.Context,
	cmd ReconcilePeriodCommand,
	diffs []Diff,
) ([]LeakageCandidate, error) {
	candidates := make([]LeakageCandidate, 0, len(diffs))

	for _, diff := range diffs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if diff.LeakageAmount.MinorUnits <= cmd.MinimumLeakageAmount.MinorUnits {
			continue
		}

		caseType := leakage.CaseTypeUnderbilling
		if diff.ActualAmount.IsZero() {
			caseType = leakage.CaseTypeUnbilledUsage
		}

		candidates = append(candidates, LeakageCandidate{
			ID:              newCaseID(),
			Type:            caseType,
			Severity:        severityForAmount(diff.LeakageAmount),
			Diff:            diff,
			ConfidenceScore: valueobject.MustConfidenceScore(10_000),
			TraceID:         cmd.TraceID,
		})
	}

	return candidates, nil
}

// TODO: вынести в глобальные константы
func severityForAmount(amount valueobject.Money) leakage.Severity {
	switch {
	case amount.MinorUnits >= 1_000_000:
		return leakage.SeverityCritical
	case amount.MinorUnits >= 100_000:
		return leakage.SeverityHigh
	case amount.MinorUnits >= 10_000:
		return leakage.SeverityMedium
	default:
		return leakage.SeverityLow
	}
}
