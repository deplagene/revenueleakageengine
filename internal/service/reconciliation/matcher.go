package reconciliation

import (
	"errors"
	"fmt"

	"github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
)

// Matcher groups expected and actual revenue entries by reconciliation key.
type Matcher interface {
	Match(
		expected []revenue.ExpectedRevenueEntry,
		actual []revenue.ActualRevenueEntry,
	) ([]MatchedEntry, error)
}

type defaultMatcher struct{}

// NewMatcher returns the default matcher for expected-vs-actual ledger entries.
func NewMatcher() Matcher {
	return defaultMatcher{}
}

// Match groups ledger entries by tenant, contract, billable item, and period.
func (m defaultMatcher) Match(
	expected []revenue.ExpectedRevenueEntry,
	actual []revenue.ActualRevenueEntry,
) ([]MatchedEntry, error) {
	matches := map[MatchKey]*MatchedEntry{}

	for _, entry := range expected {
		key := keyFromExpected(entry)
		match := ensureMatch(matches, key)
		match.Expected = append(match.Expected, entry)
	}

	for _, entry := range actual {
		key := keyFromActual(entry)
		match := ensureMatch(matches, key)
		match.Actual = append(match.Actual, entry)
	}

	result := make([]MatchedEntry, 0, len(matches))
	for _, match := range matches {
		result = append(result, *match)
	}

	return result, nil
}

// Diff computes expected, actual, and leakage amounts for a matched entry.
func (m MatchedEntry) Diff() (Diff, error) {
	currency, err := matchCurrency(m.Expected, m.Actual)
	if err != nil {
		return Diff{}, err
	}

	expectedAmount, err := sumExpected(m.Expected, currency)
	if err != nil {
		return Diff{}, fmt.Errorf("sum expected amount: %w", err)
	}

	actualAmount, err := sumActual(m.Actual, currency)
	if err != nil {
		return Diff{}, fmt.Errorf("sum actual amount: %w", err)
	}

	leakageAmount, err := expectedAmount.Sub(actualAmount)
	if err != nil {
		return Diff{}, fmt.Errorf("calculate leakage amount: %w", err)
	}

	return Diff{
		Key:            m.Key,
		ExpectedAmount: expectedAmount,
		ActualAmount:   actualAmount,
		LeakageAmount:  leakageAmount,
	}, nil
}

func ensureMatch(matches map[MatchKey]*MatchedEntry, key MatchKey) *MatchedEntry {
	match, ok := matches[key]
	if ok {
		return match
	}

	match = &MatchedEntry{
		Key:      key,
		Expected: []revenue.ExpectedRevenueEntry{},
		Actual:   []revenue.ActualRevenueEntry{},
	}
	matches[key] = match

	return match
}

func keyFromExpected(entry revenue.ExpectedRevenueEntry) MatchKey {
	return MatchKey{
		TenantID:       entry.TenantID,
		CustomerID:     entry.CustomerID,
		ContractID:     entry.ContractID,
		BillableItemID: entry.BillableItemID,
		Period:         entry.Period,
	}
}

func keyFromActual(entry revenue.ActualRevenueEntry) MatchKey {
	return MatchKey{
		TenantID:       entry.TenantID,
		CustomerID:     entry.CustomerID,
		ContractID:     entry.ContractID,
		BillableItemID: entry.BillableItemID,
		Period:         entry.Period,
	}
}

func matchCurrency(
	expected []revenue.ExpectedRevenueEntry,
	actual []revenue.ActualRevenueEntry,
) (string, error) {
	if len(expected) > 0 {
		return expected[0].ExpectedAmount.Currency, nil
	}

	if len(actual) > 0 {
		return actual[0].ActualAmount.Currency, nil
	}

	return "", errors.New("matched entry has no revenue entries")
}

func sumExpected(entries []revenue.ExpectedRevenueEntry, currency string) (valueobject.Money, error) {
	total := valueobject.ZeroMoney(currency)
	for _, entry := range entries {
		next, err := total.Add(entry.ExpectedAmount)
		if err != nil {
			return valueobject.Money{}, err
		}

		total = next
	}

	return total, nil
}

func sumActual(entries []revenue.ActualRevenueEntry, currency string) (valueobject.Money, error) {
	total := valueobject.ZeroMoney(currency)
	for _, entry := range entries {
		next, err := total.Add(entry.ActualAmount)
		if err != nil {
			return valueobject.Money{}, err
		}

		total = next
	}

	return total, nil
}
