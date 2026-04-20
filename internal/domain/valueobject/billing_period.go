package valueobject

import (
	"errors"
	"time"
)

var ErrInvalidBillingPeriod = errors.New("invalid billing period")

type BillingPeriod struct {
	Start time.Time
	End   time.Time
}

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

func (p BillingPeriod) Validate() error {
	if p.Start.IsZero() || p.End.IsZero() {
		return ErrInvalidBillingPeriod
	}

	if !p.Start.Before(p.End) {
		return ErrInvalidBillingPeriod
	}

	return nil
}

func (p BillingPeriod) Contains(at time.Time) bool {
	at = at.UTC()
	return !at.Before(p.Start) && at.Before(p.End)
}
