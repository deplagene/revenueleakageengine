// Package revenue contains business operations for expected and actual revenue
// construction.
package revenue

import (
	"context"
	"time"

	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/google/uuid"
)

// Option configures a revenue service dependency.
type Option func(*Service)

// Service coordinates revenue calculation workflows.
type Service struct {
	calculator           Calculator
	actualRevenueBuilder ActualRevenueBuilder
	store                Store
	clock                func() time.Time
}

// NewService creates a revenue service with deterministic default
// dependencies.
func NewService(options ...Option) *Service {
	service := &Service{
		calculator:           NewFixedUsageCalculator(),
		actualRevenueBuilder: NewInvoiceActualRevenueBuilder(),
		clock:                time.Now,
	}

	for _, option := range options {
		option(service)
	}

	return service
}

// WithStore configures the persistence dependency used by state-changing
// revenue workflows.
func WithStore(store Store) Option {
	return func(service *Service) {
		if store != nil {
			service.store = store
		}
	}
}

// WithActualRevenueBuilder replaces the actual revenue builder.
func WithActualRevenueBuilder(builder ActualRevenueBuilder) Option {
	return func(service *Service) {
		if builder != nil {
			service.actualRevenueBuilder = builder
		}
	}
}

// WithCalculator replaces the expected revenue calculator.
func WithCalculator(calculator Calculator) Option {
	return func(service *Service) {
		if calculator != nil {
			service.calculator = calculator
		}
	}
}

// WithClock replaces the clock used for generated calculation timestamps.
func WithClock(clock func() time.Time) Option {
	return func(service *Service) {
		if clock != nil {
			service.clock = clock
		}
	}
}

// CalculateExpectedRevenue produces an expected revenue ledger entry from the
// command inputs. Missing entry id and calculation time are filled at the
// service boundary.
func (s *Service) CalculateExpectedRevenue(
	ctx context.Context,
	cmd CalculateExpectedRevenueCommand,
) (revenuedomain.ExpectedRevenueEntry, error) {
	if err := ctx.Err(); err != nil {
		return revenuedomain.ExpectedRevenueEntry{}, err
	}

	if cmd.EntryID == uuid.Nil {
		cmd.EntryID = uuid.New()
	}

	if cmd.CalculatedAt.IsZero() {
		cmd.CalculatedAt = s.clock().UTC()
	}

	return s.calculator.CalculateExpectedRevenue(cmd)
}

// BuildActualRevenue produces invoice-based actual revenue ledger entries from
// normalized billing facts.
func (s *Service) BuildActualRevenue(
	ctx context.Context,
	cmd BuildActualRevenueCommand,
) ([]revenuedomain.ActualRevenueEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if cmd.RecognizedAt.IsZero() {
		cmd.RecognizedAt = s.clock().UTC()
	}

	return s.actualRevenueBuilder.BuildActualRevenue(cmd)
}

// CalculateAndSaveExpectedRevenue calculates an expected revenue entry and
// persists it as a ledger fact.
func (s *Service) CalculateAndSaveExpectedRevenue(
	ctx context.Context,
	cmd CalculateExpectedRevenueCommand,
) (revenuedomain.ExpectedRevenueEntry, error) {
	if s.store == nil {
		return revenuedomain.ExpectedRevenueEntry{}, ErrStoreRequired
	}

	entry, err := s.CalculateExpectedRevenue(ctx, cmd)
	if err != nil {
		return revenuedomain.ExpectedRevenueEntry{}, err
	}

	if err := s.store.SaveExpectedRevenue(ctx, entry); err != nil {
		return revenuedomain.ExpectedRevenueEntry{}, err
	}

	return entry, nil
}

// BuildAndSaveActualRevenue builds invoice-based actual revenue entries and
// persists them as ledger facts.
func (s *Service) BuildAndSaveActualRevenue(
	ctx context.Context,
	cmd BuildActualRevenueCommand,
) ([]revenuedomain.ActualRevenueEntry, error) {
	if s.store == nil {
		return nil, ErrStoreRequired
	}

	entries, err := s.BuildActualRevenue(ctx, cmd)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if err := s.store.SaveActualRevenue(ctx, entry); err != nil {
			return nil, err
		}
	}

	return entries, nil
}
