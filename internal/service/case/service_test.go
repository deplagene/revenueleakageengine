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
	cases           []leakage.Case
	received        ListCasesCommand
	caseResult      GetCaseResult
	caseErr         error
	receivedCase    GetCaseCommand
	updatedCase     leakage.Case
	history         leakage.StatusHistory
	updateErr       error
	updatedAssignee leakage.Case
	updateAssignErr error
}

func (f *fakeStore) ListCases(ctx context.Context, cmd ListCasesCommand) ([]leakage.Case, error) {
	f.received = cmd
	return f.cases, nil
}

func (f *fakeStore) GetCase(ctx context.Context, cmd GetCaseCommand) (GetCaseResult, error) {
	f.receivedCase = cmd
	return f.caseResult, f.caseErr
}

func (f *fakeStore) UpdateCaseStatus(
	ctx context.Context,
	c leakage.Case,
	history leakage.StatusHistory,
) error {
	f.updatedCase = c
	f.history = history
	return f.updateErr
}

func (f *fakeStore) UpdateCaseAssignee(ctx context.Context, c leakage.Case) error {
	f.updatedAssignee = c
	return f.updateAssignErr
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

func TestServiceUpdateCaseStatus(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	caseID := uuid.New()
	store := &fakeStore{
		caseResult: GetCaseResult{
			Case: leakage.Case{
				ID:       caseID,
				TenantID: tenantID,
				Status:   leakage.StatusOpen,
			},
		},
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.UpdateCaseStatus(context.Background(), UpdateCaseStatusCommand{
		TenantID:  tenantID,
		CaseID:    caseID,
		Status:    leakage.StatusInvestigating,
		ChangedBy: "billing-ops",
	})
	if err != nil {
		t.Fatalf("UpdateCaseStatus() error = %v", err)
	}

	if result.Case.Status != leakage.StatusInvestigating {
		t.Fatalf("status = %s, want %s", result.Case.Status, leakage.StatusInvestigating)
	}

	if store.updatedCase.Status != leakage.StatusInvestigating {
		t.Fatalf("persisted status = %s, want %s", store.updatedCase.Status, leakage.StatusInvestigating)
	}

	if store.history.FromStatus != leakage.StatusOpen {
		t.Fatalf("from status = %s, want %s", store.history.FromStatus, leakage.StatusOpen)
	}

	if store.history.ToStatus != leakage.StatusInvestigating {
		t.Fatalf("to status = %s, want %s", store.history.ToStatus, leakage.StatusInvestigating)
	}
}

func TestServiceUpdateCaseStatusInvalidTransition(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		caseResult: GetCaseResult{
			Case: leakage.Case{
				ID:       uuid.New(),
				TenantID: uuid.New(),
				Status:   leakage.StatusResolved,
			},
		},
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = service.UpdateCaseStatus(context.Background(), UpdateCaseStatusCommand{
		TenantID: store.caseResult.Case.TenantID,
		CaseID:   store.caseResult.Case.ID,
		Status:   leakage.StatusDismissed,
	})
	if !errors.Is(err, leakage.ErrInvalidStatusTransition) {
		t.Fatalf("UpdateCaseStatus() error = %v, want ErrInvalidStatusTransition", err)
	}
}

func TestServiceUpdateCaseStatusNoOp(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	caseID := uuid.New()
	store := &fakeStore{
		caseResult: GetCaseResult{
			Case: leakage.Case{
				ID:       caseID,
				TenantID: tenantID,
				Status:   leakage.StatusOpen,
			},
		},
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.UpdateCaseStatus(context.Background(), UpdateCaseStatusCommand{
		TenantID: tenantID,
		CaseID:   caseID,
		Status:   leakage.StatusOpen,
	})
	if err != nil {
		t.Fatalf("UpdateCaseStatus() error = %v", err)
	}

	if result.Case.Status != leakage.StatusOpen {
		t.Fatalf("status = %s, want %s", result.Case.Status, leakage.StatusOpen)
	}

	if store.updatedCase.ID != uuid.Nil {
		t.Fatalf("expected no persistence call, got updated case id %s", store.updatedCase.ID)
	}
}

func TestServiceUpdateCaseAssignee(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	caseID := uuid.New()
	store := &fakeStore{
		caseResult: GetCaseResult{
			Case: leakage.Case{
				ID:       caseID,
				TenantID: tenantID,
				Assignee: "billing-triage",
			},
		},
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.UpdateCaseAssignee(context.Background(), UpdateCaseAssigneeCommand{
		TenantID: tenantID,
		CaseID:   caseID,
		Assignee: "billing-ops",
	})
	if err != nil {
		t.Fatalf("UpdateCaseAssignee() error = %v", err)
	}

	if result.Case.Assignee != "billing-ops" {
		t.Fatalf("assignee = %q, want %q", result.Case.Assignee, "billing-ops")
	}

	if store.updatedAssignee.Assignee != "billing-ops" {
		t.Fatalf("persisted assignee = %q, want %q", store.updatedAssignee.Assignee, "billing-ops")
	}
}

func TestServiceUpdateCaseAssigneeNoOp(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	caseID := uuid.New()
	store := &fakeStore{
		caseResult: GetCaseResult{
			Case: leakage.Case{
				ID:       caseID,
				TenantID: tenantID,
				Assignee: "billing-ops",
			},
		},
	}

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.UpdateCaseAssignee(context.Background(), UpdateCaseAssigneeCommand{
		TenantID: tenantID,
		CaseID:   caseID,
		Assignee: "billing-ops",
	})
	if err != nil {
		t.Fatalf("UpdateCaseAssignee() error = %v", err)
	}

	if result.Case.Assignee != "billing-ops" {
		t.Fatalf("assignee = %q, want %q", result.Case.Assignee, "billing-ops")
	}

	if store.updatedAssignee.ID != uuid.Nil {
		t.Fatalf("expected no assignee persistence call, got updated case id %s", store.updatedAssignee.ID)
	}
}
