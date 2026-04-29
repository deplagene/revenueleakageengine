package contract

import (
	"context"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/contract"
	contractservice "github.com/deplagene/revenueleakageengine/internal/service/contract"
	"github.com/google/uuid"
)

// Commands orchestrates contract modification use cases.
type Commands struct {
	service *contractservice.Service
}

// NewCommands creates a new commands handler.
func NewCommands(service *contractservice.Service) *Commands {
	return &Commands{service: service}
}

func (c *Commands) UpsertContract(ctx context.Context, con *contract.Contract) error {
	return c.service.UpsertContract(ctx, con)
}

func (c *Commands) UpsertBillableItem(ctx context.Context, item *contract.BillableItem) error {
	return c.service.UpsertBillableItem(ctx, item)
}

func (c *Commands) UpsertTerm(ctx context.Context, term *contract.Term) error {
	return c.service.UpsertTerm(ctx, term)
}

// Queries orchestrates contract retrieval use cases.
type Queries struct {
	service *contractservice.Service
}

// NewQueries creates a new queries handler.
func NewQueries(service *contractservice.Service) *Queries {
	return &Queries{service: service}
}

func (q *Queries) GetContract(ctx context.Context, id uuid.UUID) (*contract.Contract, error) {
	return q.service.GetContract(ctx, id)
}

func (q *Queries) GetBillableItemByCode(ctx context.Context, tenantID uuid.UUID, code string) (*contract.BillableItem, error) {
	return q.service.GetBillableItemByCode(ctx, tenantID, code)
}

func (q *Queries) ListContractsByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]*contract.Contract, error) {
	return q.service.ListContractsByCustomer(ctx, tenantID, customerID)
}

func (q *Queries) GetEffectiveTerms(ctx context.Context, contractID uuid.UUID, at time.Time) ([]*contract.Term, error) {
	return q.service.GetEffectiveTerms(ctx, contractID, at)
}
