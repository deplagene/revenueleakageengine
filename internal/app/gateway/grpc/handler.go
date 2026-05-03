// Package grpc contains gRPC delivery handlers and proto/domain mapping for the gateway.
package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	revenueleakageenginev1 "github.com/deplagene/revenueleakageengine/internal/gen/proto/revenueleakageengine/v1"
	casework "github.com/deplagene/revenueleakageengine/internal/service/case"
	contractwork "github.com/deplagene/revenueleakageengine/internal/service/contract"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	"github.com/google/uuid"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ErrReconciliationRunnerRequired reports that gRPC routes were created
// without a reconciliation application dependency.
var ErrReconciliationRunnerRequired = errors.New("reconciliation runner is required")

// ErrCaseQueriesRequired reports that gRPC routes were created without a case
// query dependency.
var ErrCaseQueriesRequired = errors.New("case queries are required")

type reconciliationRunner interface {
	RunRevenueLeakageCheck(
		ctx context.Context,
		cmd appreconciliation.RunRevenueLeakageCheckCommand,
	) (appreconciliation.RunRevenueLeakageCheckResult, error)
	ListReconciliationRuns(
		ctx context.Context,
		cmd appreconciliation.ListReconciliationRunsCommand,
	) (appreconciliation.ListReconciliationRunsResult, error)
	GetReconciliationRun(
		ctx context.Context,
		cmd appreconciliation.GetReconciliationRunCommand,
	) (appreconciliation.GetReconciliationRunResult, error)
}

type caseQueries interface {
	ListCases(
		ctx context.Context,
		cmd caseapp.ListCasesCommand,
	) (caseapp.ListCasesResult, error)
}

// Handler implements the generated gRPC service and maps proto DTOs to app
// use-case commands.
type Handler struct {
	revenueleakageenginev1.UnimplementedReconciliationServiceServer

	reconciliation reconciliationRunner
	cases          caseQueries
}

// NewHandler constructs the gateway gRPC handler set.
func NewHandler(reconciliation reconciliationRunner, cases caseQueries) (*Handler, error) {
	if reconciliation == nil {
		return nil, ErrReconciliationRunnerRequired
	}

	if cases == nil {
		return nil, ErrCaseQueriesRequired
	}

	return &Handler{
		reconciliation: reconciliation,
		cases:          cases,
	}, nil
}

// Register attaches the service implementation to a gRPC registrar.
func (h *Handler) Register(registrar googlegrpc.ServiceRegistrar) {
	revenueleakageenginev1.RegisterReconciliationServiceServer(registrar, h)
}

// Run starts a full expected-vs-actual reconciliation workflow.
func (h *Handler) Run(
	ctx context.Context,
	req *revenueleakageenginev1.RunRequest,
) (*revenueleakageenginev1.RunResponse, error) {
	cmd, err := runCommandFromProto(req)
	if err != nil {
		return nil, invalidArgument(err)
	}

	result, err := h.reconciliation.RunRevenueLeakageCheck(ctx, cmd)
	if err != nil {
		return nil, rpcError(err)
	}

	return runResponseToProto(result), nil
}

// GetRun loads one persisted reconciliation run.
func (h *Handler) GetRun(
	ctx context.Context,
	req *revenueleakageenginev1.GetRunRequest,
) (*revenueleakageenginev1.GetRunResponse, error) {
	cmd, err := getRunCommandFromProto(req)
	if err != nil {
		return nil, invalidArgument(err)
	}

	result, err := h.reconciliation.GetReconciliationRun(ctx, cmd)
	if err != nil {
		return nil, rpcError(err)
	}

	return &revenueleakageenginev1.GetRunResponse{
		Run: reconciliationRunToProto(result.Run),
	}, nil
}

// ListCases loads leakage cases in tenant scope.
func (h *Handler) ListCases(
	ctx context.Context,
	req *revenueleakageenginev1.ListCasesRequest,
) (*revenueleakageenginev1.ListCasesResponse, error) {
	cmd, err := listCasesCommandFromProto(req)
	if err != nil {
		return nil, invalidArgument(err)
	}

	result, err := h.cases.ListCases(ctx, cmd)
	if err != nil {
		return nil, rpcError(err)
	}

	return &revenueleakageenginev1.ListCasesResponse{
		Cases: leakageCasesToProto(result.Cases),
	}, nil
}

func runCommandFromProto(
	req *revenueleakageenginev1.RunRequest,
) (appreconciliation.RunRevenueLeakageCheckCommand, error) {
	if req == nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, errors.New("request is required")
	}

	tenantID, err := requiredUUID("tenant_id", req.GetTenantId())
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	contractID, err := requiredUUID("contract_id", req.GetContractId())
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	period, err := billingPeriodFromProto(req.GetPeriod())
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	runID, err := optionalUUID("run_id", req.GetRunId())
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	minimumLeakage, err := minimumLeakageFromProto(req.GetCurrency(), req.GetMinimumLeakageMinorUnits())
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, err
	}

	return appreconciliation.RunRevenueLeakageCheckCommand{
		TenantID:             tenantID,
		ContractID:           contractID,
		PeriodStart:          period.Start,
		PeriodEnd:            period.End,
		RunID:                runID,
		TraceID:              strings.TrimSpace(req.GetTraceId()),
		Currency:             strings.TrimSpace(req.GetCurrency()),
		MinimumLeakageAmount: minimumLeakage,
	}, nil
}

func getRunCommandFromProto(
	req *revenueleakageenginev1.GetRunRequest,
) (appreconciliation.GetReconciliationRunCommand, error) {
	if req == nil {
		return appreconciliation.GetReconciliationRunCommand{}, errors.New("request is required")
	}

	tenantID, err := requiredUUID("tenant_id", req.GetTenantId())
	if err != nil {
		return appreconciliation.GetReconciliationRunCommand{}, err
	}

	runID, err := requiredUUID("run_id", req.GetRunId())
	if err != nil {
		return appreconciliation.GetReconciliationRunCommand{}, err
	}

	return appreconciliation.GetReconciliationRunCommand{
		TenantID: tenantID,
		RunID:    runID,
	}, nil
}

func listCasesCommandFromProto(
	req *revenueleakageenginev1.ListCasesRequest,
) (caseapp.ListCasesCommand, error) {
	if req == nil {
		return caseapp.ListCasesCommand{}, errors.New("request is required")
	}

	tenantID, err := requiredUUID("tenant_id", req.GetTenantId())
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	contractID, err := optionalUUID("contract_id", req.GetContractId())
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	statusValue, err := caseStatusFromProto(req.GetStatus())
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	severity, err := severityFromProto(req.GetSeverity())
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	detectedFrom, err := optionalTimestamp("detected_from", req.GetDetectedFrom())
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	detectedTo, err := optionalTimestamp("detected_to", req.GetDetectedTo())
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	return caseapp.ListCasesCommand{
		TenantID:     tenantID,
		ContractID:   contractID,
		Status:       statusValue,
		Severity:     severity,
		DetectedFrom: detectedFrom,
		DetectedTo:   detectedTo,
		Search:       strings.TrimSpace(req.GetSearch()),
		Limit:        int(req.GetLimit()),
		Offset:       int(req.GetOffset()),
	}, nil
}

func runResponseToProto(
	result appreconciliation.RunRevenueLeakageCheckResult,
) *revenueleakageenginev1.RunResponse {
	reconciliationResult := result.ReconciliationResult

	return &revenueleakageenginev1.RunResponse{
		RunId:            reconciliationResult.RunID.String(),
		ExpectedEntryId:  result.ExpectedEntry.String(),
		ActualEntryCount: int64(result.ActualEntryCount),
		DiffCount:        int64(reconciliationResult.DiffCount),
		CaseCount:        int64(reconciliationResult.CaseCount),
		LeakageAmount:    moneyToProto(reconciliationResult.LeakageAmount),
		Cases:            leakageCasesToProto(reconciliationResult.Cases),
	}
}

func reconciliationRunToProto(
	run reconciliationservice.ReconciliationRunSummary,
) *revenueleakageenginev1.ReconciliationRun {
	return &revenueleakageenginev1.ReconciliationRun{
		Id:            run.ID.String(),
		TenantId:      run.TenantID.String(),
		ContractId:    run.ContractID.String(),
		Period:        billingPeriodToProto(run.Period),
		Status:        runStatusToProto(run.Status),
		StartedAt:     timestampOrNil(run.StartedAt),
		CompletedAt:   timestampOrNil(run.CompletedAt),
		ExpectedCount: run.ExpectedCount,
		ActualCount:   run.ActualCount,
		DiffCount:     run.DiffCount,
		CaseCount:     run.CaseCount,
		LeakageAmount: moneyToProto(run.LeakageAmount),
		TraceId:       run.TraceID,
	}
}

func leakageCasesToProto(cases []leakage.Case) []*revenueleakageenginev1.LeakageCase {
	items := make([]*revenueleakageenginev1.LeakageCase, 0, len(cases))
	for _, item := range cases {
		items = append(items, leakageCaseToProto(item))
	}

	return items
}

func leakageCaseToProto(c leakage.Case) *revenueleakageenginev1.LeakageCase {
	return &revenueleakageenginev1.LeakageCase{
		Id:                    c.ID.String(),
		TenantId:              c.TenantID.String(),
		CustomerId:            c.CustomerID.String(),
		ContractId:            c.ContractID.String(),
		ReconciliationRunId:   c.ReconciliationRunID.String(),
		Type:                  string(c.Type),
		Severity:              severityToProto(c.Severity),
		Status:                caseStatusToProto(c.Status),
		DetectedAt:            timestampOrNil(c.DetectedAt),
		Period:                billingPeriodToProto(c.Period),
		ExpectedAmount:        moneyToProto(c.ExpectedAmount),
		ActualAmount:          moneyToProto(c.ActualAmount),
		LeakageAmount:         moneyToProto(c.LeakageAmount),
		ConfidenceBasisPoints: confidenceBasisPointsToProto(c.ConfidenceScore),
		RootCauseCategory:     string(c.RootCauseCategory),
		Assignee:              c.Assignee,
		TraceId:               c.TraceID,
	}
}

func moneyToProto(money valueobject.Money) *revenueleakageenginev1.Money {
	return &revenueleakageenginev1.Money{
		Currency:   money.Currency,
		MinorUnits: money.MinorUnits,
	}
}

func billingPeriodFromProto(protoPeriod *revenueleakageenginev1.BillingPeriod) (valueobject.BillingPeriod, error) {
	if protoPeriod == nil {
		return valueobject.BillingPeriod{}, errors.New("period is required")
	}

	start, err := requiredTimestamp("period.start", protoPeriod.GetStart())
	if err != nil {
		return valueobject.BillingPeriod{}, err
	}

	end, err := requiredTimestamp("period.end", protoPeriod.GetEnd())
	if err != nil {
		return valueobject.BillingPeriod{}, err
	}

	period, err := valueobject.NewBillingPeriod(start, end)
	if err != nil {
		return valueobject.BillingPeriod{}, fmt.Errorf("period is invalid: %w", err)
	}

	return period, nil
}

func billingPeriodToProto(period valueobject.BillingPeriod) *revenueleakageenginev1.BillingPeriod {
	return &revenueleakageenginev1.BillingPeriod{
		Start: timestampOrNil(period.Start),
		End:   timestampOrNil(period.End),
	}
}

func minimumLeakageFromProto(currency string, minorUnits int64) (valueobject.Money, error) {
	currency = strings.TrimSpace(currency)
	if currency == "" && minorUnits == 0 {
		return valueobject.Money{}, nil
	}

	if currency == "" {
		return valueobject.Money{}, errors.New("currency is required when minimum_leakage_minor_units is set")
	}

	money, err := valueobject.NewMoney(currency, minorUnits)
	if err != nil {
		return valueobject.Money{}, fmt.Errorf("minimum_leakage_minor_units is invalid: %w", err)
	}

	return money, nil
}

func requiredUUID(field, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, fmt.Errorf("%s is required", field)
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s is invalid: %w", field, err)
	}

	return id, nil
}

func optionalUUID(field, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, nil
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s is invalid: %w", field, err)
	}

	return id, nil
}

func requiredTimestamp(field string, value *timestamppb.Timestamp) (time.Time, error) {
	if value == nil {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}

	if err := value.CheckValid(); err != nil {
		return time.Time{}, fmt.Errorf("%s is invalid: %w", field, err)
	}

	return value.AsTime().UTC(), nil
}

func optionalTimestamp(field string, value *timestamppb.Timestamp) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}

	if err := value.CheckValid(); err != nil {
		return time.Time{}, fmt.Errorf("%s is invalid: %w", field, err)
	}

	return value.AsTime().UTC(), nil
}

func timestampOrNil(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}

	return timestamppb.New(value.UTC())
}

func runStatusToProto(statusValue reconciliationservice.ReconciliationRunStatus) revenueleakageenginev1.ReconciliationRunStatus {
	switch statusValue {
	case reconciliationservice.ReconciliationRunStatusRunning:
		return revenueleakageenginev1.ReconciliationRunStatus_RECONCILIATION_RUN_STATUS_RUNNING
	case reconciliationservice.ReconciliationRunStatusCompleted:
		return revenueleakageenginev1.ReconciliationRunStatus_RECONCILIATION_RUN_STATUS_COMPLETED
	default:
		return revenueleakageenginev1.ReconciliationRunStatus_RECONCILIATION_RUN_STATUS_UNSPECIFIED
	}
}

func caseStatusFromProto(statusValue revenueleakageenginev1.LeakageCaseStatus) (leakage.Status, error) {
	switch statusValue {
	case revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_UNSPECIFIED:
		return "", nil
	case revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_OPEN:
		return leakage.StatusOpen, nil
	case revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_INVESTIGATING:
		return leakage.StatusInvestigating, nil
	case revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_RESOLVED:
		return leakage.StatusResolved, nil
	case revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_DISMISSED:
		return leakage.StatusDismissed, nil
	default:
		return "", fmt.Errorf("status is unsupported: %s", statusValue.String())
	}
}

func caseStatusToProto(statusValue leakage.Status) revenueleakageenginev1.LeakageCaseStatus {
	switch statusValue {
	case leakage.StatusOpen:
		return revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_OPEN
	case leakage.StatusInvestigating:
		return revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_INVESTIGATING
	case leakage.StatusResolved:
		return revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_RESOLVED
	case leakage.StatusDismissed:
		return revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_DISMISSED
	default:
		return revenueleakageenginev1.LeakageCaseStatus_LEAKAGE_CASE_STATUS_UNSPECIFIED
	}
}

func severityFromProto(statusValue revenueleakageenginev1.LeakageSeverity) (leakage.Severity, error) {
	switch statusValue {
	case revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_UNSPECIFIED:
		return "", nil
	case revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_LOW:
		return leakage.SeverityLow, nil
	case revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_MEDIUM:
		return leakage.SeverityMedium, nil
	case revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_HIGH:
		return leakage.SeverityHigh, nil
	case revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_CRITICAL:
		return leakage.SeverityCritical, nil
	default:
		return "", fmt.Errorf("severity is unsupported: %s", statusValue.String())
	}
}

func severityToProto(severity leakage.Severity) revenueleakageenginev1.LeakageSeverity {
	switch severity {
	case leakage.SeverityLow:
		return revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_LOW
	case leakage.SeverityMedium:
		return revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_MEDIUM
	case leakage.SeverityHigh:
		return revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_HIGH
	case leakage.SeverityCritical:
		return revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_CRITICAL
	default:
		return revenueleakageenginev1.LeakageSeverity_LEAKAGE_SEVERITY_UNSPECIFIED
	}
}

func confidenceBasisPointsToProto(score valueobject.ConfidenceScore) int32 {
	return int32(score.BasisPoints) //nolint:gosec // uint16 always fits into int32.
}

func invalidArgument(err error) error {
	return status.Error(codes.InvalidArgument, err.Error())
}

func rpcError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case isNotFoundError(err):
		return status.Error(codes.NotFound, err.Error())
	case isInvalidArgumentError(err):
		return invalidArgument(err)
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

func isNotFoundError(err error) bool {
	return errors.Is(err, reconciliationservice.ErrRunNotFound) ||
		errors.Is(err, caseapp.ErrCaseNotFound) ||
		errors.Is(err, contractwork.ErrContractNotFound) ||
		errors.Is(err, contractwork.ErrBillableItemNotFound)
}

func isInvalidArgumentError(err error) bool {
	return errors.Is(err, billing.ErrTenantRequired) ||
		errors.Is(err, billing.ErrContractRequired) ||
		errors.Is(err, contract.ErrBillableItemIDRequired) ||
		errors.Is(err, valueobject.ErrInvalidBillingPeriod) ||
		errors.Is(err, valueobject.ErrCurrencyRequired) ||
		errors.Is(err, valueobject.ErrCurrencyMismatch) ||
		errors.Is(err, casework.ErrTenantRequired) ||
		errors.Is(err, casework.ErrLimitInvalid) ||
		errors.Is(err, casework.ErrOffsetInvalid) ||
		errors.Is(err, casework.ErrSeverityInvalid) ||
		errors.Is(err, reconciliationservice.ErrTenantRequired) ||
		errors.Is(err, reconciliationservice.ErrContractRequired) ||
		errors.Is(err, reconciliationservice.ErrCurrencyRequired) ||
		errors.Is(err, reconciliationservice.ErrLimitInvalid) ||
		errors.Is(err, reconciliationservice.ErrRunIDRequired)
}
