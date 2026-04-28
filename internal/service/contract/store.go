package contract

// Store contracts for this package belong here, defined from the service's
// actual read/write needs rather than from storage implementation details.

import (
	"context"
	"time"

	contractdomain "github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/google/uuid"
)

// Store defines the persistence operations for contracts and terms.
type Store interface {
	GetContract(ctx context.Context, id uuid.UUID) (*contractdomain.Contract, error)
	UpsertContract(ctx context.Context, c *contractdomain.Contract) error
	ListContractsByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]*contractdomain.Contract, error)

	GetBillableItem(ctx context.Context, id uuid.UUID) (*contractdomain.BillableItem, error)
	GetBillableItemByCode(ctx context.Context, tenantID uuid.UUID, code string) (*contractdomain.BillableItem, error)
	UpsertBillableItem(ctx context.Context, item *contractdomain.BillableItem) error
	ListBillableItems(ctx context.Context, tenantID uuid.UUID) ([]*contractdomain.BillableItem, error)

	UpsertTerm(ctx context.Context, term *contractdomain.Term) error
	ListEffectiveTerms(ctx context.Context, contractID uuid.UUID, at time.Time) ([]*contractdomain.Term, error)
}
