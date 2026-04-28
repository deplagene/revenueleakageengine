package contract

import (
	"context"
	"errors"
	"testing"
	"time"

	contractdomain "github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/google/uuid"
)

func TestNewServiceRequiresStore(t *testing.T) {
	t.Parallel()

	_, err := NewService(nil)
	if !errors.Is(err, ErrStoreRequired) {
		t.Fatalf("NewService(nil) error = %v, want %v", err, ErrStoreRequired)
	}
}

func TestServiceUpsertContractNormalizesAndGeneratesID(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	contract := &contractdomain.Contract{
		TenantID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CustomerID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Status:       contractdomain.StatusActive,
		StartDate:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Currency:     " usd ",
		BillingModel: contractdomain.BillingModelFixedRecurring,
	}

	if err := service.UpsertContract(context.Background(), contract); err != nil {
		t.Fatalf("UpsertContract() error = %v", err)
	}

	if contract.ID == uuid.Nil {
		t.Fatal("contract id was not generated")
	}

	if contract.Currency != "USD" {
		t.Fatalf("contract currency = %q, want %q", contract.Currency, "USD")
	}

	if contract.Metadata == nil {
		t.Fatal("contract metadata is nil")
	}

	if contract.Version != 1 {
		t.Fatalf("contract version = %d, want 1", contract.Version)
	}

	if store.contract == nil || store.contract.ID != contract.ID {
		t.Fatalf("stored contract id = %v, want %v", store.contract, contract.ID)
	}
}

func TestServiceUpsertContractValidatesInput(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	contract := &contractdomain.Contract{
		CustomerID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Status:       contractdomain.StatusActive,
		StartDate:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Currency:     "USD",
		Version:      1,
		BillingModel: contractdomain.BillingModelFixedRecurring,
	}

	err = service.UpsertContract(context.Background(), contract)
	if !errors.Is(err, contractdomain.ErrTenantRequired) {
		t.Fatalf("UpsertContract() error = %v, want %v", err, contractdomain.ErrTenantRequired)
	}

	if store.contract != nil {
		t.Fatal("invalid contract was persisted")
	}
}

func TestServiceUpsertTermDerivesTenantFromContract(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	contractID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	store := &recordingStore{
		contract: &contractdomain.Contract{
			ID:           contractID,
			TenantID:     tenantID,
			CustomerID:   uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			Status:       contractdomain.StatusActive,
			StartDate:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			Currency:     "USD",
			Version:      1,
			BillingModel: contractdomain.BillingModelFixedRecurring,
			Metadata:     map[string]any{},
		},
	}
	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	term := &contractdomain.Term{
		ContractID:    contractID,
		Type:          contractdomain.TermTypeFixedFee,
		EffectiveFrom: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Expression: map[string]any{
			"code":               "platform_subscription",
			"amount_minor_units": int64(50000),
			"currency":           "USD",
		},
	}

	if err := service.UpsertTerm(context.Background(), term); err != nil {
		t.Fatalf("UpsertTerm() error = %v", err)
	}

	if term.ID == uuid.Nil {
		t.Fatal("term id was not generated")
	}

	if term.TenantID != tenantID {
		t.Fatalf("term tenant id = %s, want %s", term.TenantID, tenantID)
	}

	if store.term == nil || store.term.TenantID != tenantID {
		t.Fatalf("stored term tenant id = %v, want %s", store.term, tenantID)
	}
}

type recordingStore struct {
	contract *contractdomain.Contract
	item     *contractdomain.BillableItem
	term     *contractdomain.Term
}

func (s *recordingStore) GetContract(
	_ context.Context,
	id uuid.UUID,
) (*contractdomain.Contract, error) {
	if s.contract == nil || s.contract.ID != id {
		return nil, ErrContractNotFound
	}

	return s.contract, nil
}

func (s *recordingStore) UpsertContract(_ context.Context, contract *contractdomain.Contract) error {
	clone := *contract
	s.contract = &clone
	return nil
}

func (s *recordingStore) ListContractsByCustomer(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) ([]*contractdomain.Contract, error) {
	return []*contractdomain.Contract{}, nil
}

func (s *recordingStore) GetBillableItem(
	context.Context,
	uuid.UUID,
) (*contractdomain.BillableItem, error) {
	return nil, ErrBillableItemNotFound
}

func (s *recordingStore) GetBillableItemByCode(
	context.Context,
	uuid.UUID,
	string,
) (*contractdomain.BillableItem, error) {
	return nil, ErrBillableItemNotFound
}

func (s *recordingStore) UpsertBillableItem(_ context.Context, item *contractdomain.BillableItem) error {
	clone := *item
	s.item = &clone
	return nil
}

func (s *recordingStore) ListBillableItems(
	context.Context,
	uuid.UUID,
) ([]*contractdomain.BillableItem, error) {
	return []*contractdomain.BillableItem{}, nil
}

func (s *recordingStore) UpsertTerm(_ context.Context, term *contractdomain.Term) error {
	clone := *term
	s.term = &clone
	return nil
}

func (s *recordingStore) ListEffectiveTerms(
	context.Context,
	uuid.UUID,
	time.Time,
) ([]*contractdomain.Term, error) {
	return []*contractdomain.Term{}, nil
}
