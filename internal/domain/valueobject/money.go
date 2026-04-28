package valueobject

import (
	"errors"
	"strings"
)

var (
	// ErrCurrencyRequired reports that a money value was created without a currency code.
	ErrCurrencyRequired = errors.New("currency is required")
	// ErrCurrencyMismatch reports that an arithmetic operation was attempted across currencies.
	ErrCurrencyMismatch = errors.New("currency mismatch")
)

// Money stores a currency and amount in minor units to keep revenue arithmetic
// deterministic and auditable.
type Money struct {
	Currency   string
	MinorUnits int64
}

// NewMoney normalizes the currency code to upper case and constructs a money
// value in minor units.
func NewMoney(currency string, minorUnits int64) (Money, error) {
	normalizedCurrency := strings.ToUpper(strings.TrimSpace(currency))
	if normalizedCurrency == "" {
		return Money{}, ErrCurrencyRequired
	}

	return Money{
		Currency:   normalizedCurrency,
		MinorUnits: minorUnits,
	}, nil
}

// MustMoney constructs a money value and panics if the currency is invalid.
// Use it for trusted bootstrap data and tests, not for user input.
func MustMoney(currency string, minorUnits int64) Money {
	money, err := NewMoney(currency, minorUnits)
	if err != nil {
		panic(err)
	}

	return money
}

// Validate checks that the money value carries a normalized currency code.
func (m Money) Validate() error {
	if strings.TrimSpace(m.Currency) == "" {
		return ErrCurrencyRequired
	}

	return nil
}

// ZeroMoney returns a zero-valued money amount in the requested currency.
func ZeroMoney(currency string) Money {
	return MustMoney(currency, 0)
}

// Add sums two money values with the same currency.
func (m Money) Add(other Money) (Money, error) {
	if !m.SameCurrency(other) {
		return Money{}, ErrCurrencyMismatch
	}

	return Money{
		Currency:   m.Currency,
		MinorUnits: m.MinorUnits + other.MinorUnits,
	}, nil
}

// Sub subtracts one money value from another when both share the same currency.
func (m Money) Sub(other Money) (Money, error) {
	if !m.SameCurrency(other) {
		return Money{}, ErrCurrencyMismatch
	}

	return Money{
		Currency:   m.Currency,
		MinorUnits: m.MinorUnits - other.MinorUnits,
	}, nil
}

// Negate returns the same money value with the sign inverted.
func (m Money) Negate() Money {
	return Money{
		Currency:   m.Currency,
		MinorUnits: -m.MinorUnits,
	}
}

// IsZero reports whether the money amount equals zero minor units.
func (m Money) IsZero() bool {
	return m.MinorUnits == 0
}

// SameCurrency reports whether two money values use the same currency code.
func (m Money) SameCurrency(other Money) bool {
	return m.Currency == other.Currency
}
