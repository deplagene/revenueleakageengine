package contract

import (
	"context"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/google/uuid"
)

// Service coordinates contract business rules.
type Service struct {
	store Store
}

// NewService creates a new contract service.
func NewService(store Store) *Service {
	return &Service{
		store: store,
	}
}

// GetContract returns a contract by its ID.
func (s *Service) GetContract(ctx context.Context, id uuid.UUID) (*contract.Contract, error) {
	return s.store.GetContract(ctx, id)
}

// UpsertContract creates or updates a contract.
func (s *Service) UpsertContract(ctx context.Context, c *contract.Contract) error {
	// Add validation here if needed
	return s.store.UpsertContract(ctx, c)
}

// ListContractsByCustomer returns all contracts for a specific customer.
func (s *Service) ListContractsByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]*contract.Contract, error) {
	return s.store.ListContractsByCustomer(ctx, tenantID, customerID)
}

// GetBillableItem returns a billable item by its ID.
func (s *Service) GetBillableItem(ctx context.Context, id uuid.UUID) (*contract.BillableItem, error) {
	return s.store.GetBillableItem(ctx, id)
}

// UpsertBillableItem creates or updates a billable item.
func (s *Service) UpsertBillableItem(ctx context.Context, item *contract.BillableItem) error {
	return s.store.UpsertBillableItem(ctx, item)
}

// UpsertTerm adds or updates a contract term.
func (s *Service) UpsertTerm(ctx context.Context, t *contract.Term) error {
	return s.store.UpsertTerm(ctx, t)
}

// GetEffectiveTerms returns all contract terms active at the given time.
func (s *Service) GetEffectiveTerms(ctx context.Context, contractID uuid.UUID, at time.Time) ([]*contract.Term, error) {
	return s.store.ListEffectiveTerms(ctx, contractID, at)
}
