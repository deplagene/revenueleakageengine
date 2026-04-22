package reconciliation

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/google/uuid"
)

// EvidenceBuilder creates proof records for generated leakage cases.
type EvidenceBuilder interface {
	Build(c leakage.Case, candidate LeakageCandidate, now func() time.Time) []leakage.Evidence
}

type defaultEvidenceBuilder struct{}

// NewEvidenceBuilder returns the default evidence builder for reconciliation
// diffs.
func NewEvidenceBuilder() EvidenceBuilder {
	return defaultEvidenceBuilder{}
}

// Build creates a compact invoice-gap evidence payload from the calculated
// expected-vs-actual diff.
func (b defaultEvidenceBuilder) Build(
	c leakage.Case,
	candidate LeakageCandidate,
	now func() time.Time,
) []leakage.Evidence {
	return []leakage.Evidence{
		{
			ID:         uuid.New(),
			CaseID:     c.ID,
			Type:       leakage.EvidenceTypeInvoiceGap,
			EntityType: "revenue_diff",
			EntityID:   candidate.Diff.Key.BillableItemID.String(),
			Payload: map[string]any{
				"expected_minor_units": candidate.Diff.ExpectedAmount.MinorUnits,
				"actual_minor_units":   candidate.Diff.ActualAmount.MinorUnits,
				"leakage_minor_units":  candidate.Diff.LeakageAmount.MinorUnits,
				"currency":             candidate.Diff.LeakageAmount.Currency,
				"period_start":         candidate.Diff.Key.Period.Start,
				"period_end":           candidate.Diff.Key.Period.End,
			},
			CreatedAt: now().UTC(),
		},
	}
}
