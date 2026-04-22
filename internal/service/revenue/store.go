package revenue

import (
	"context"
	"errors"

	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
)

// ErrStoreRequired reports that a service workflow needs persistence but no
// store was configured.
var ErrStoreRequired = errors.New("revenue store is required")

// Store defines persistence required by revenue workflows.
//
// The interface stays in this package because revenue owns the use case and
// decides which entries it needs to persist.
type Store interface {
	SaveExpectedRevenue(ctx context.Context, entry revenuedomain.ExpectedRevenueEntry) error
	SaveActualRevenue(ctx context.Context, entry revenuedomain.ActualRevenueEntry) error
}
