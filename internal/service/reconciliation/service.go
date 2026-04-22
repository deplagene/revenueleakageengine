// Package reconciliation contains business operations that compare expected
// revenue against actual revenue and produce leakage candidates.
package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// ErrStoreRequired reports that a reconciliation service was created without
// a store implementation.
var ErrStoreRequired = errors.New("reconciliation store is required")

// Service coordinates reconciliation business workflows.
type Service struct {
	store           Store
	matcher         Matcher
	detector        Detector
	evidenceBuilder EvidenceBuilder
	now             func() time.Time
}

// Option customizes service dependencies used by tests and future rule sets.
type Option func(*Service)

// WithMatcher replaces the default expected-vs-actual matcher.
func WithMatcher(matcher Matcher) Option {
	return func(s *Service) {
		if matcher != nil {
			s.matcher = matcher
		}
	}
}

// WithDetector replaces the default leakage detector.
func WithDetector(detector Detector) Option {
	return func(s *Service) {
		if detector != nil {
			s.detector = detector
		}
	}
}

// WithEvidenceBuilder replaces the default evidence builder.
func WithEvidenceBuilder(builder EvidenceBuilder) Option {
	return func(s *Service) {
		if builder != nil {
			s.evidenceBuilder = builder
		}
	}
}

// WithClock replaces the service clock. Use it in tests to keep detected_at and
// evidence timestamps deterministic.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// NewService constructs a reconciliation service with deterministic default
// matcher, detector, and evidence builder.
func NewService(store Store, opts ...Option) (*Service, error) {
	if store == nil {
		return nil, ErrStoreRequired
	}

	service := &Service{
		store:           store,
		matcher:         NewMatcher(),
		detector:        NewDetector(),
		evidenceBuilder: NewEvidenceBuilder(),
		now:             time.Now,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service, nil
}

// ReconcilePeriod compares expected and actual revenue for one contract period,
// detects leakage candidates, and persists resulting cases with evidence.
func (s *Service) ReconcilePeriod(
	ctx context.Context,
	cmd ReconcilePeriodCommand,
) (ReconcilePeriodResult, error) {
	const op = "service.reconciliation.ReconcilePeriod"

	if err := cmd.Validate(); err != nil {
		return ReconcilePeriodResult{}, fmt.Errorf("%s: %w", op, err)
	}

	expected, err := s.store.ListExpectedRevenue(
		ctx,
		cmd.TenantID,
		cmd.ContractID,
		cmd.Period,
	)
	if err != nil {
		return ReconcilePeriodResult{}, fmt.Errorf("%s: list expected revenue: %w", op, err)
	}

	actual, err := s.store.ListActualRevenue(
		ctx,
		cmd.TenantID,
		cmd.ContractID,
		cmd.Period,
	)
	if err != nil {
		return ReconcilePeriodResult{}, fmt.Errorf("%s: list actual revenue: %w", op, err)
	}

	matches, err := s.matcher.Match(expected, actual)
	if err != nil {
		return ReconcilePeriodResult{}, fmt.Errorf("%s: match revenue entries: %w", op, err)
	}

	diffs := make([]Diff, 0, len(matches))
	for _, match := range matches {
		diff, err := match.Diff()
		if err != nil {
			return ReconcilePeriodResult{}, fmt.Errorf("%s: build diff: %w", op, err)
		}

		diffs = append(diffs, diff)
	}

	candidates, err := s.detector.Detect(ctx, cmd, diffs)
	if err != nil {
		return ReconcilePeriodResult{}, fmt.Errorf("%s: detect leakage: %w", op, err)
	}

	result := ReconcilePeriodResult{
		RunID:         cmd.RunID,
		ExpectedCount: len(expected),
		ActualCount:   len(actual),
		DiffCount:     len(diffs),
		CaseCount:     len(candidates),
		Cases:         make([]leakage.Case, 0, len(candidates)),
	}

	for _, candidate := range candidates {
		c := candidate.ToCase(s.now())
		evidence := s.evidenceBuilder.Build(c, candidate, s.now)

		if err := s.store.CreateLeakageCase(ctx, c, evidence); err != nil {
			return ReconcilePeriodResult{}, fmt.Errorf("%s: create leakage case: %w", op, err)
		}

		result.Cases = append(result.Cases, c)

		if result.LeakageAmount.IsZero() {
			result.LeakageAmount = c.LeakageAmount
			continue
		}

		result.LeakageAmount, err = result.LeakageAmount.Add(c.LeakageAmount)
		if err != nil {
			return ReconcilePeriodResult{}, fmt.Errorf("%s: sum leakage amount: %w", op, err)
		}
	}

	if result.LeakageAmount.Currency == "" {
		result.LeakageAmount = valueobject.ZeroMoney(cmd.Currency)
	}

	return result, nil
}

func newCaseID() uuid.UUID {
	return uuid.New()
}
