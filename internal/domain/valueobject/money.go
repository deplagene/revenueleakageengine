package valueobject

import (
	"errors"
	"strings"
)

var (
	ErrCurrencyRequired = errors.New("currency is required")
	ErrCurrencyMismatch = errors.New("currency mismatch")
)

type Money struct {
	Currency   string
	MinorUnits int64
}

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

func MustMoney(currency string, minorUnits int64) Money {
	money, err := NewMoney(currency, minorUnits)
	if err != nil {
		// ? точно нужна ли паника
		panic(err)
	}

	return money
}

func ZeroMoney(currency string) Money {
	return MustMoney(currency, 0)
}

func (m Money) Add(other Money) (Money, error) {
	if !m.SameCurrency(other) {
		return Money{}, ErrCurrencyMismatch
	}

	return Money{
		Currency:   m.Currency,
		MinorUnits: m.MinorUnits + other.MinorUnits,
	}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if !m.SameCurrency(other) {
		return Money{}, ErrCurrencyMismatch
	}

	return Money{
		Currency:   m.Currency,
		MinorUnits: m.MinorUnits - other.MinorUnits,
	}, nil
}

func (m Money) Negate() Money {
	return Money{
		Currency:   m.Currency,
		MinorUnits: -m.MinorUnits,
	}
}

func (m Money) IsZero() bool {
	return m.MinorUnits == 0
}

func (m Money) SameCurrency(other Money) bool {
	return m.Currency == other.Currency
}
