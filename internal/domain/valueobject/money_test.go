package valueobject

import "testing"

func TestMoneyAdd(t *testing.T) {
	left := MustMoney("usd", 1_000)
	right := MustMoney("USD", 250)

	got, err := left.Add(right)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Currency != "USD" {
		t.Fatalf("unexpected currency: %s", got.Currency)
	}

	if got.MinorUnits != 1_250 {
		t.Fatalf("unexpected amount: %d", got.MinorUnits)
	}
}

func TestMoneyAddCurrencyMismatch(t *testing.T) {
	left := MustMoney("USD", 1_000)
	right := MustMoney("EUR", 1_000)

	if _, err := left.Add(right); err == nil {
		t.Fatal("expected currency mismatch error")
	}
}
