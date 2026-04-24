package casework

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

type fakeStore struct {
	cases        []leakage.Case
	received     ListCasesCommand
	caseResult   GetCaseResult
	caseErr      error
	receivedCase GetCaseCommand
}

func (f *fakeStore) ListCases(ctx context.Context, cmd ListCasesCommand) ([]leakage.Case, error) {
	f.received = cmd
	return f.cases, nil
}

func (f *fakeStore) GetCase(ctx context.Context, cmd GetCaseCommand) (GetCaseResult, error) {
	f.receivedCase = cmd
	return f.caseResult, f.caseErr
}

func TestServiceListCases(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	store := &fakeStore{
		cases: []leakage.Case{
			{
				ID:              uuid.New(),
				TenantID:        tenantID,
				Type:            leakage.CaseTypeUnderbilling,
				Severity:        leakage.SeverityHigh,
				Status:          leakage.StatusOpen,
				DetectedAt:      time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC),
				ExpectedAmount:  valueobject.MustMoney("USD", 5800),
				ActualAmount:    valueobject.MustMoney("USD", 4640),
				LeakageAmount:   valueobject.MustMoney("USD", 1160),
				ConfidenceScore: valueobject.MustConfidenceScore(9000),
			},
		},
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.ListCases(context.Background(), ListCasesCommand{
		TenantID: tenantID,
	})
	if err != nil {
		t.Fatalf("ListCases() error = %v", err)
	}

	if got := len(result.Cases); got != 1 {
		t.Fatalf("case count = %d, want 1", got)
	}

	if store.received.Limit != defaultListLimit {
		t.Fatalf("limit = %d, want %d", store.received.Limit, defaultListLimit)
	}
}

func TestServiceGetCase(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	caseID := uuid.New()
	store := &fakeStore{
		caseResult: GetCaseResult{
			Case: leakage.Case{
				ID:       caseID,
				TenantID: tenantID,
			},
		},
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.GetCase(context.Background(), GetCaseCommand{
		TenantID: tenantID,
		CaseID:   caseID,
	})
	if err != nil {
		t.Fatalf("GetCase() error = %v", err)
	}

	if result.Case.ID != caseID {
		t.Fatalf("case id = %s, want %s", result.Case.ID, caseID)
	}
}

func TestServiceGetCaseNotFound(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		caseErr: ErrCaseNotFound,
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = service.GetCase(context.Background(), GetCaseCommand{
		TenantID: uuid.New(),
		CaseID:   uuid.New(),
	})
	if !errors.Is(err, ErrCaseNotFound) {
		t.Fatalf("GetCase() error = %v, want ErrCaseNotFound", err)
	}
}
