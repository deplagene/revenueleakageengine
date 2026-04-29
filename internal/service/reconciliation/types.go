package reconciliation

import (
	"errors"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// ReconciliationRunStatus captures the persisted lifecycle of one run.
type ReconciliationRunStatus string

const (
	ReconciliationRunStatusRunning   ReconciliationRunStatus = "running"
	ReconciliationRunStatusCompleted ReconciliationRunStatus = "completed"
)

var (
	// ErrTenantRequired reports that a reconciliation command has no tenant.
	ErrTenantRequired = errors.New("tenant id is required")
	// ErrContractRequired reports that a reconciliation command has no contract.
	ErrContractRequired = errors.New("contract id is required")
	// ErrCurrencyRequired reports that a reconciliation command has no currency.
	ErrCurrencyRequired = errors.New("currency is required")
	// ErrLimitInvalid reports that a read-model query has an unsupported page
	// size.
	ErrLimitInvalid = errors.New("limit must be between 1 and 100")
)

const (
	defaultRunsLimit = 10
	maxRunsLimit     = 100
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

// ReconciliationRun stores the persisted execution statistics for one
// reconciliation pass.
type ReconciliationRun struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	ContractID    uuid.UUID
	Period        valueobject.BillingPeriod
	Status        ReconciliationRunStatus
	StartedAt     time.Time
	CompletedAt   time.Time
	ExpectedCount int
	ActualCount   int
	DiffCount     int
	CaseCount     int
	LeakageAmount valueobject.Money
	Currency      string
	TraceID       string
}

// ListReconciliationRunsCommand requests a page of recent reconciliation runs.
type ListReconciliationRunsCommand struct {
	TenantID   uuid.UUID
	ContractID uuid.UUID
	Limit      int
	Offset     int
}

// Normalize applies safe pagination defaults.
func (c ListReconciliationRunsCommand) Normalize() ListReconciliationRunsCommand {
	if c.Limit == 0 {
		c.Limit = defaultRunsLimit
	}

	return c
}

// Validate checks read-model pagination bounds.
func (c ListReconciliationRunsCommand) Validate() error {
	if c.Limit < 1 || c.Limit > maxRunsLimit {
		return ErrLimitInvalid
	}

	if c.Offset < 0 {
		return ErrLimitInvalid
	}

	return nil
}

// ListReconciliationRunsResult contains one page of reconciliation run
// summaries.
type ListReconciliationRunsResult struct {
	Runs []ReconciliationRunSummary
}

// ReconciliationRunSummary is the read model used by dashboards and lists.
type ReconciliationRunSummary struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	ContractID    uuid.UUID
	Period        valueobject.BillingPeriod
	Status        ReconciliationRunStatus
	StartedAt     time.Time
	CompletedAt   time.Time
	ExpectedCount int64
	ActualCount   int64
	DiffCount     int64
	CaseCount     int64
	LeakageAmount valueobject.Money
	TraceID       string
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
