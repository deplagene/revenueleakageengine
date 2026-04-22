package revenue

import "github.com/deplagene/revenueleakageengine/internal/domain/valueobject"

// Diff summarizes the gap between expected and actual revenue for one
// reconciliation unit.
type Diff struct {
	ExpectedAmount valueobject.Money
	ActualAmount   valueobject.Money
	LeakageAmount  valueobject.Money
}
