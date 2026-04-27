package contract

// Store contracts for this package belong here, defined from the service's
// actual read/write needs rather than from storage implementation details.

import (
	"context"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/google/uuid"
)

// Store defines the persistence operations for contracts and terms.
type Store interface {
	GetContract(ctx context.Context, id uuid.UUID) (*contract.Contract, error)
	UpsertContract(ctx context.Context, c *contract.Contract) error
	ListContractsByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]*contract.Contract, error)

	GetBillableItem(ctx context.Context, id uuid.UUID) (*contract.BillableItem, error)
	GetBillableItemByCode(ctx context.Context, tenantID uuid.UUID, code string) (*contract.BillableItem, error)
	UpsertBillableItem(ctx context.Context, item *contract.BillableItem) error
	ListBillableItems(ctx context.Context, tenantID uuid.UUID) ([]*contract.BillableItem, error)

	UpsertTerm(ctx context.Context, term *contract.Term) error
	ListEffectiveTerms(ctx context.Context, contractID uuid.UUID, at time.Time) ([]*contract.Term, error)
}
