package valueobject

import (
	"errors"
	"time"
)

// ErrInvalidBillingPeriod reports that a billing period is empty or has a
// non-increasing time range.
var ErrInvalidBillingPeriod = errors.New("invalid billing period")

// BillingPeriod defines an inclusive-exclusive time window used for billing and
// reconciliation.
type BillingPeriod struct {
	Start time.Time
	End   time.Time
}

// NewBillingPeriod normalizes a billing period to UTC and validates that the
// start is strictly before the end.
func NewBillingPeriod(start, end time.Time) (BillingPeriod, error) {
	start = start.UTC()
	end = end.UTC()

	period := BillingPeriod{
		Start: start,
		End:   end,
	}

	if err := period.Validate(); err != nil {
		return BillingPeriod{}, err
	}

	return period, nil
}

// Validate ensures that both boundaries are set and that the period has a
// positive duration.
func (p BillingPeriod) Validate() error {
	if p.Start.IsZero() || p.End.IsZero() {
		return ErrInvalidBillingPeriod
	}

	if !p.Start.Before(p.End) {
		return ErrInvalidBillingPeriod
	}

	return nil
}

// Contains reports whether a timestamp falls within the period using an
// inclusive start and exclusive end.
func (p BillingPeriod) Contains(at time.Time) bool {
	at = at.UTC()
	return !at.Before(p.Start) && at.Before(p.End)
}
