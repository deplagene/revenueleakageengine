package valueobject

import (
	"testing"
	"time"
)

func TestNewBillingPeriod(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)

	period, err := NewBillingPeriod(start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !period.Contains(start.Add(24 * time.Hour)) {
		t.Fatal("expected timestamp to be inside billing period")
	}

	if period.Contains(end) {
		t.Fatal("expected end timestamp to be exclusive")
	}
}

func TestNewBillingPeriodRejectsInvalidRange(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)

	if _, err := NewBillingPeriod(start, start); err == nil {
		t.Fatal("expected invalid billing period error")
	}
}
