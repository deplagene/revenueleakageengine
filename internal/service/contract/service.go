package contract

import (
	"context"
	"fmt"
	"time"

	contractdomain "github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/google/uuid"
)

// Service coordinates contract business rules.
type Service struct {
	store Store
}

// NewService creates a new contract service.
func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, ErrStoreRequired
	}

	return &Service{
		store: store,
	}, nil
}

// GetContract returns a contract by its ID.
func (s *Service) GetContract(ctx context.Context, id uuid.UUID) (*contractdomain.Contract, error) {
	const op = "service.contract.GetContract"

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if id == uuid.Nil {
		return nil, fmt.Errorf("%s: %w", op, contractdomain.ErrContractIDRequired)
	}

	c, err := s.store.GetContract(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return c, nil
}

// UpsertContract creates or updates a contract.
func (s *Service) UpsertContract(ctx context.Context, c *contractdomain.Contract) error {
	const op = "service.contract.UpsertContract"

	if err := ctx.Err(); err != nil {
		return err
	}

	if c == nil {
		return fmt.Errorf("%s: %w", op, ErrContractRequired)
	}

	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}

	if c.Version == 0 {
		c.Version = 1
	}

	normalized := c.Normalize()
	if err := normalized.Validate(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	*c = normalized
	if err := s.store.UpsertContract(ctx, c); err != nil {
		return fmt.Errorf("%s: upsert contract: %w", op, err)
	}

	return nil
}

// ListContractsByCustomer returns all contracts for a specific customer.
func (s *Service) ListContractsByCustomer(
	ctx context.Context,
	tenantID uuid.UUID,
	customerID uuid.UUID,
) ([]*contractdomain.Contract, error) {
	const op = "service.contract.ListContractsByCustomer"

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("%s: %w", op, contractdomain.ErrTenantRequired)
	}

	if customerID == uuid.Nil {
		return nil, fmt.Errorf("%s: %w", op, contractdomain.ErrCustomerRequired)
	}

	contracts, err := s.store.ListContractsByCustomer(ctx, tenantID, customerID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return contracts, nil
}

// GetBillableItem returns a billable item by its ID.
func (s *Service) GetBillableItem(ctx context.Context, id uuid.UUID) (*contractdomain.BillableItem, error) {
	const op = "service.contract.GetBillableItem"

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if id == uuid.Nil {
		return nil, fmt.Errorf("%s: %w", op, contractdomain.ErrBillableItemIDRequired)
	}

	item, err := s.store.GetBillableItem(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return item, nil
}

// UpsertBillableItem creates or updates a billable item.
func (s *Service) UpsertBillableItem(ctx context.Context, item *contractdomain.BillableItem) error {
	const op = "service.contract.UpsertBillableItem"

	if err := ctx.Err(); err != nil {
		return err
	}

	if item == nil {
		return fmt.Errorf("%s: %w", op, ErrBillableItemRequired)
	}

	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}

	normalized := item.Normalize()
	if err := normalized.Validate(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	*item = normalized
	if err := s.store.UpsertBillableItem(ctx, item); err != nil {
		return fmt.Errorf("%s: upsert billable item: %w", op, err)
	}

	return nil
}

// UpsertTerm adds or updates a contract term.
func (s *Service) UpsertTerm(ctx context.Context, t *contractdomain.Term) error {
	const op = "service.contract.UpsertTerm"

	if err := ctx.Err(); err != nil {
		return err
	}

	if t == nil {
		return fmt.Errorf("%s: %w", op, ErrTermRequired)
	}

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	if t.ContractID == uuid.Nil {
		return fmt.Errorf("%s: %w", op, contractdomain.ErrContractIDRequired)
	}

	if t.TenantID == uuid.Nil {
		c, err := s.store.GetContract(ctx, t.ContractID)
		if err != nil {
			return fmt.Errorf("%s: get contract tenant: %w", op, err)
		}

		t.TenantID = c.TenantID
	}

	normalized := t.Normalize()
	if err := normalized.Validate(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	*t = normalized
	if err := s.store.UpsertTerm(ctx, t); err != nil {
		return fmt.Errorf("%s: upsert term: %w", op, err)
	}

	return nil
}

// GetEffectiveTerms returns all contract terms active at the given time.
func (s *Service) GetEffectiveTerms(
	ctx context.Context,
	contractID uuid.UUID,
	at time.Time,
) ([]*contractdomain.Term, error) {
	const op = "service.contract.GetEffectiveTerms"

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if contractID == uuid.Nil {
		return nil, fmt.Errorf("%s: %w", op, contractdomain.ErrContractIDRequired)
	}

	if at.IsZero() {
		at = time.Now().UTC()
	}

	terms, err := s.store.ListEffectiveTerms(ctx, contractID, at)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return terms, nil
}
