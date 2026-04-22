package reconciliation

import (
	"errors"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

var (
	// ErrTenantRequired reports that a reconciliation command has no tenant.
	ErrTenantRequired = errors.New("tenant id is required")
	// ErrContractRequired reports that a reconciliation command has no contract.
	ErrContractRequired = errors.New("contract id is required")
	// ErrCurrencyRequired reports that a reconciliation command has no currency.
	ErrCurrencyRequired = errors.New("currency is required")
)

// ReconcilePeriodCommand starts reconciliation for one tenant, contract, and
// billing period.
type ReconcilePeriodCommand struct {
	TenantID             uuid.UUID
	ContractID           uuid.UUID
	Period               valueobject.BillingPeriod
	RunID                uuid.UUID
	TraceID              string
	Currency             string
	MinimumLeakageAmount valueobject.Money
}

// Validate checks that a reconciliation command has enough business context to
// produce traceable results.
func (c ReconcilePeriodCommand) Validate() error {
	if c.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if c.ContractID == uuid.Nil {
		return ErrContractRequired
	}

	if err := c.Period.Validate(); err != nil {
		return err
	}

	if c.Currency == "" {
		return ErrCurrencyRequired
	}

	return nil
}

// ReconcilePeriodResult summarizes a reconciliation run.
type ReconcilePeriodResult struct {
	RunID         uuid.UUID
	ExpectedCount int
	ActualCount   int
	DiffCount     int
	CaseCount     int
	LeakageAmount valueobject.Money
	Cases         []leakage.Case
}

// MatchKey identifies the reconciliation unit used to compare expected and
// actual revenue.
type MatchKey struct {
	TenantID       uuid.UUID
	CustomerID     uuid.UUID
	ContractID     uuid.UUID
	BillableItemID uuid.UUID
	Period         valueobject.BillingPeriod
}

// MatchedEntry groups expected and actual ledger entries for one match key.
type MatchedEntry struct {
	Key      MatchKey
	Expected []revenue.ExpectedRevenueEntry
	Actual   []revenue.ActualRevenueEntry
}

// Diff captures the calculated revenue gap for one match key.
type Diff struct {
	Key            MatchKey
	ExpectedAmount valueobject.Money
	ActualAmount   valueobject.Money
	LeakageAmount  valueobject.Money
}

// LeakageCandidate is a detector output before it is persisted as a case.
type LeakageCandidate struct {
	ID              uuid.UUID
	Type            leakage.CaseType
	Severity        leakage.Severity
	Diff            Diff
	ConfidenceScore valueobject.ConfidenceScore
	TraceID         string
}

// ToCase converts a leakage candidate into the domain case entity.
func (c LeakageCandidate) ToCase(detectedAt time.Time) leakage.Case {
	return leakage.Case{
		ID:              c.ID,
		TenantID:        c.Diff.Key.TenantID,
		CustomerID:      c.Diff.Key.CustomerID,
		ContractID:      c.Diff.Key.ContractID,
		Type:            c.Type,
		Severity:        c.Severity,
		Status:          leakage.StatusOpen,
		DetectedAt:      detectedAt.UTC(),
		Period:          c.Diff.Key.Period,
		ExpectedAmount:  c.Diff.ExpectedAmount,
		ActualAmount:    c.Diff.ActualAmount,
		LeakageAmount:   c.Diff.LeakageAmount,
		ConfidenceScore: c.ConfidenceScore,
		TraceID:         c.TraceID,
	}
}
