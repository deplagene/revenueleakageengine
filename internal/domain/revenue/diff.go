package revenue

import "github.com/deplagene/revenueleakageengine/internal/domain/valueobject"

type Diff struct {
	ExpectedAmount valueobject.Money
	ActualAmount   valueobject.Money
	LeakageAmount  valueobject.Money
}
