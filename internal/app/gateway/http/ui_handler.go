package http

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	caseapp "github.com/deplagene/revenueleakageengine/internal/app/case"
	"github.com/deplagene/revenueleakageengine/internal/app/gateway/http/ui"
	appreconciliation "github.com/deplagene/revenueleakageengine/internal/app/reconciliation"
	"github.com/deplagene/revenueleakageengine/internal/domain/leakage"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	reconciliationservice "github.com/deplagene/revenueleakageengine/internal/service/reconciliation"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

//go:embed static/*
var uiStaticFS embed.FS

const recentRunsLimit = 8

func (h *Handler) registerUIRoutes(router chi.Router) {
	router.Handle(
		"/ui/static/*",
		stdhttp.StripPrefix("/ui/", stdhttp.FileServer(stdhttp.FS(uiStaticFS))),
	)
	router.Get("/ui", h.handleUIDashboard)
	router.Get("/ui/reconciliation", h.handleUIReconciliation)
	router.Post("/ui/reconciliation/run", h.handleUIRunReconciliation)
	router.Get("/ui/cases", h.handleUICases)
	router.Get("/ui/cases/{case_id}", h.handleUICaseDetail)
	router.Post("/ui/cases/{case_id}/status", h.handleUICaseStatus)
	router.Post("/ui/cases/{case_id}/assignee", h.handleUICaseAssignee)
}

func (h *Handler) handleUIDashboard(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	renderUI(w, r, ui.DashboardPage(h.dashboardPageData(r)))
}

func (h *Handler) handleUIReconciliation(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	data := h.dashboardPageData(r)
	data.Title = "Запуск сверки"

	renderUI(w, r, ui.ReconciliationPage(data))
}

func (h *Handler) handleUIRunReconciliation(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	cmd, tenantID, err := runReconciliationCommandFromForm(r)
	if err != nil {
		renderUI(w, r, ui.RunResult(ui.RunResultData{Error: err.Error()}))
		return
	}

	result, err := h.reconciliation.RunRevenueLeakageCheck(r.Context(), cmd)
	if err != nil {
		renderUI(w, r, ui.RunResult(ui.RunResultData{Error: err.Error()}))
		return
	}

	renderUI(w, r, ui.RunResult(newRunResultData(tenantID, result)))
}

func (h *Handler) handleUICases(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	table := h.casesTableData(r, defaultCasesLimit)
	if isHTMX(r) {
		renderUI(w, r, ui.CasesTable(table))
		return
	}

	renderUI(w, r, ui.CasesPage(newCasesPageData(table)))
}

func (h *Handler) handleUICaseDetail(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	panel := h.caseDetailPanelData(r)
	if isHTMX(r) {
		renderUI(w, r, ui.CaseDetailPanel(panel))
		return
	}

	renderUI(w, r, ui.CaseDetailPage(ui.CaseDetailPageData{
		Title: "Кейс утечки",
		Panel: panel,
	}))
}

func (h *Handler) handleUICaseStatus(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if err := r.ParseForm(); err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	tenantID, err := parseUUID("tenant id", r.PostForm.Get("tenant_id"))
	if err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	caseID, err := parseUUID("case id", chi.URLParam(r, "case_id"))
	if err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	status, err := parseCaseStatus(r.PostForm.Get("status"))
	if err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	if _, err := h.caseCommands.UpdateCaseStatus(r.Context(), caseapp.UpdateCaseStatusCommand{
		TenantID:   tenantID,
		CaseID:     caseID,
		Status:     status,
		ChangedBy:  strings.TrimSpace(r.PostForm.Get("changed_by")),
		ReasonCode: strings.TrimSpace(r.PostForm.Get("reason_code")),
		Comment:    strings.TrimSpace(r.PostForm.Get("comment")),
	}); err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	renderUI(w, r, ui.CaseDetailPanel(h.caseDetailPanelForIDs(r.Context(), tenantID, caseID)))
}

func (h *Handler) handleUICaseAssignee(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if err := r.ParseForm(); err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	tenantID, err := parseUUID("tenant id", r.PostForm.Get("tenant_id"))
	if err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	caseID, err := parseUUID("case id", chi.URLParam(r, "case_id"))
	if err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	if _, err := h.caseCommands.UpdateCaseAssignee(r.Context(), caseapp.UpdateCaseAssigneeCommand{
		TenantID: tenantID,
		CaseID:   caseID,
		Assignee: strings.TrimSpace(r.PostForm.Get("assignee")),
	}); err != nil {
		renderUI(w, r, ui.CaseDetailPanel(ui.CaseDetailPanelData{Error: err.Error()}))
		return
	}

	renderUI(w, r, ui.CaseDetailPanel(h.caseDetailPanelForIDs(r.Context(), tenantID, caseID)))
}

func renderUI(w stdhttp.ResponseWriter, r *stdhttp.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(stdhttp.StatusOK)

	if err := component.Render(r.Context(), w); err != nil {
		stdhttp.Error(w, "render ui", stdhttp.StatusInternalServerError)
	}
}

func isHTMX(r *stdhttp.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (h *Handler) dashboardPageData(r *stdhttp.Request) ui.DashboardPageData {
	tenantRaw := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	data := ui.DashboardPageData{
		Title:              "Операционная консоль выручки",
		TenantID:           tenantRaw,
		CompletedRunsLabel: "0",
		OpenCasesLabel:     "0",
		LastRunLabel:       "нет запусков",
		TotalLeakageLabel:  ui.FormatMoney("USD", 0),
		RunForm:            defaultRunFormData(tenantRaw),
		RecentCases: ui.CasesTableData{
			TenantID:     tenantRaw,
			EmptyMessage: "Укажите tenant ID, чтобы загрузить открытые кейсы.",
		},
	}

	runsCommand, err := listRunsCommandFromRequest(r)
	if err != nil {
		data.Error = err.Error()
	} else {
		runsResult, err := h.reconciliation.ListReconciliationRuns(r.Context(), runsCommand)
		if err != nil {
			data.Error = err.Error()
			return data
		}

		data.Runs = newRunRows(runsResult.Runs)
		data.CompletedRunsLabel = ui.FormatCount(int64(len(runsResult.Runs)))
		data.TotalLeakageLabel = totalLeakageLabel(runsResult.Runs)
		if len(runsResult.Runs) > 0 {
			periodMonth := runsResult.Runs[0].Period.End.AddDate(0, 0, -1)
			data.LastRunLabel = "Сверка за " + ui.FormatMonthYear(periodMonth)
		}
	}

	if tenantRaw != "" {
		casesQuery := r.Clone(r.Context())
		values := casesQuery.URL.Query()
		values.Set("tenant_id", tenantRaw)
		values.Set("status", string(leakage.StatusOpen))
		casesQuery.URL.RawQuery = values.Encode()

		data.RecentCases = h.casesTableData(casesQuery, 6)
		data.OpenCasesLabel = ui.FormatCount(int64(len(data.RecentCases.Cases)))
	}

	return data
}

func defaultRunFormData(tenantID string) ui.RunFormData {
	return ui.RunFormData{
		TenantID:                 tenantID,
		PeriodStart:              "2026-04-01",
		PeriodEnd:                "2026-05-01",
		Currency:                 "USD",
		MinimumLeakageMinorUnits: "0",
		TraceID:                  "ui-reconciliation",
	}
}

func listRunsCommandFromRequest(r *stdhttp.Request) (appreconciliation.ListReconciliationRunsCommand, error) {
	tenantID, err := optionalUUID("tenant id", r.URL.Query().Get("tenant_id"))
	if err != nil {
		return appreconciliation.ListReconciliationRunsCommand{}, err
	}

	contractID, err := optionalUUID("contract id", r.URL.Query().Get("contract_id"))
	if err != nil {
		return appreconciliation.ListReconciliationRunsCommand{}, err
	}

	return appreconciliation.ListReconciliationRunsCommand{
		TenantID:   tenantID,
		ContractID: contractID,
		Limit:      recentRunsLimit,
	}, nil
}

func (h *Handler) casesTableData(r *stdhttp.Request, limit int) ui.CasesTableData {
	tenantRaw := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	data := ui.CasesTableData{
		TenantID:     tenantRaw,
		ContractID:   strings.TrimSpace(r.URL.Query().Get("contract_id")),
		Status:       strings.TrimSpace(r.URL.Query().Get("status")),
		DateFrom:     strings.TrimSpace(r.URL.Query().Get("date_from")),
		DateTo:       strings.TrimSpace(r.URL.Query().Get("date_to")),
		Severity:     strings.TrimSpace(r.URL.Query().Get("severity")),
		Search:       strings.TrimSpace(r.URL.Query().Get("search")),
		EmptyMessage: "По текущему фильтру кейсов нет.",
	}
	if tenantRaw == "" {
		data.EmptyMessage = "Введите tenant ID, чтобы загрузить кейсы."
		return data
	}

	command, err := listCasesCommandFromUIRequest(r, limit)
	if err != nil {
		data.Error = err.Error()
		return data
	}

	result, err := h.cases.ListCases(r.Context(), command)
	if err != nil {
		data.Error = err.Error()
		return data
	}

	data.Cases = newCaseRows(command.TenantID, result.Cases)
	return data
}

func listCasesCommandFromUIRequest(r *stdhttp.Request, limit int) (caseapp.ListCasesCommand, error) {
	tenantID, err := parseUUID("tenant id", r.URL.Query().Get("tenant_id"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	contractID, err := optionalUUID("contract id", r.URL.Query().Get("contract_id"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	status, err := optionalCaseStatus(r.URL.Query().Get("status"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	severity, err := optionalCaseSeverity(r.URL.Query().Get("severity"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	detectedFrom, err := optionalUIDateStart("date from", r.URL.Query().Get("date_from"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	detectedTo, err := optionalUIDateEnd("date to", r.URL.Query().Get("date_to"))
	if err != nil {
		return caseapp.ListCasesCommand{}, err
	}

	return caseapp.ListCasesCommand{
		TenantID:     tenantID,
		ContractID:   contractID,
		Status:       status,
		Severity:     severity,
		DetectedFrom: detectedFrom,
		DetectedTo:   detectedTo,
		Search:       strings.TrimSpace(r.URL.Query().Get("search")),
		Limit:        limit,
	}, nil
}

func (h *Handler) caseDetailPanelData(r *stdhttp.Request) ui.CaseDetailPanelData {
	tenantID, err := parseUUID("tenant id", r.URL.Query().Get("tenant_id"))
	if err != nil {
		return ui.CaseDetailPanelData{Error: err.Error()}
	}

	caseID, err := parseUUID("case id", chi.URLParam(r, "case_id"))
	if err != nil {
		return ui.CaseDetailPanelData{Error: err.Error()}
	}

	return h.caseDetailPanelForIDs(r.Context(), tenantID, caseID)
}

func (h *Handler) caseDetailPanelForIDs(
	ctx context.Context,
	tenantID uuid.UUID,
	caseID uuid.UUID,
) ui.CaseDetailPanelData {
	result, err := h.cases.GetCase(ctx, caseapp.GetCaseCommand{
		TenantID: tenantID,
		CaseID:   caseID,
	})
	if err != nil {
		if errors.Is(err, caseapp.ErrCaseNotFound) {
			return ui.CaseDetailPanelData{Error: "Кейс не найден"}
		}

		return ui.CaseDetailPanelData{Error: err.Error()}
	}

	return newCaseDetailPanelData(tenantID, result)
}

func runReconciliationCommandFromForm(
	r *stdhttp.Request,
) (appreconciliation.RunRevenueLeakageCheckCommand, string, error) {
	if err := r.ParseForm(); err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, "", err
	}

	tenantID, err := parseUUID("tenant id", r.PostForm.Get("tenant_id"))
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, "", err
	}

	contractID, err := parseUUID("contract id", r.PostForm.Get("contract_id"))
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, "", err
	}

	periodStart, err := parseTime("period start", r.PostForm.Get("period_start"))
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, "", err
	}

	periodEnd, err := parseTime("period end", r.PostForm.Get("period_end"))
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, "", err
	}

	currency := strings.TrimSpace(r.PostForm.Get("currency"))
	minimumLeakageMinorUnits, err := optionalInt64Form("minimum leakage minor units", r.PostForm.Get("minimum_leakage_minor_units"))
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, "", err
	}

	minimumLeakageAmount, err := valueobject.NewMoney(currency, minimumLeakageMinorUnits)
	if err != nil {
		return appreconciliation.RunRevenueLeakageCheckCommand{}, "", fmt.Errorf("build minimum leakage amount: %w", err)
	}

	return appreconciliation.RunRevenueLeakageCheckCommand{
		TenantID:             tenantID,
		ContractID:           contractID,
		PeriodStart:          periodStart,
		PeriodEnd:            periodEnd,
		TraceID:              strings.TrimSpace(r.PostForm.Get("trace_id")),
		Currency:             currency,
		MinimumLeakageAmount: minimumLeakageAmount,
	}, tenantID.String(), nil
}

func newCasesPageData(table ui.CasesTableData) ui.CasesPageData {
	data := ui.CasesPageData{
		Title:                   "Кейсы",
		Table:                   table,
		OpenCasesLabel:          "0",
		InvestigatingCasesLabel: "0",
		ClosedCasesLabel:        "0",
		PotentialLeakageLabel:   ui.FormatMoney("USD", 0),
	}

	if len(table.Cases) > 0 {
		data.Preview = table.Cases[0]
	}

	var openCases int64
	var investigatingCases int64
	var closedCases int64
	var leakageTotal int64
	currency := ""
	mixedCurrency := false

	for _, item := range table.Cases {
		switch item.StatusValue {
		case string(leakage.StatusOpen):
			openCases++
		case string(leakage.StatusInvestigating):
			investigatingCases++
		case string(leakage.StatusResolved), string(leakage.StatusDismissed):
			closedCases++
		}

		if currency == "" {
			currency = item.Currency
		}
		if item.Currency != "" && item.Currency != currency {
			mixedCurrency = true
		}
		leakageTotal += item.LeakageMinorUnits
	}

	data.OpenCasesLabel = ui.FormatCount(openCases)
	data.InvestigatingCasesLabel = ui.FormatCount(investigatingCases)
	data.ClosedCasesLabel = ui.FormatCount(closedCases)
	if mixedCurrency {
		data.PotentialLeakageLabel = "разные валюты"
	} else if currency != "" {
		data.PotentialLeakageLabel = ui.FormatMoney(currency, leakageTotal)
	}

	return data
}

func optionalInt64Form(field, raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}

	return value, nil
}

func optionalUIDateStart(field, raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}

	return parseDate(field, raw)
}

func optionalUIDateEnd(field, raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}

	value, err := parseDate(field, raw)
	if err != nil {
		return time.Time{}, err
	}

	if len(raw) == len("2006-01-02") {
		value = value.AddDate(0, 0, 1)
	}

	return value, nil
}

func newRunResultData(
	tenantID string,
	result appreconciliation.RunRevenueLeakageCheckResult,
) ui.RunResultData {
	cases := newCaseRowsFromReconciliation(tenantID, result.ReconciliationResult.Cases)

	return ui.RunResultData{
		RunID:        result.ReconciliationResult.RunID.String(),
		ExpectedID:   result.ExpectedEntry.String(),
		ActualCount:  ui.FormatCount(int64(result.ActualEntryCount)),
		DiffCount:    ui.FormatCount(int64(result.ReconciliationResult.DiffCount)),
		CaseCount:    ui.FormatCount(int64(result.ReconciliationResult.CaseCount)),
		LeakageLabel: ui.FormatMoney(result.ReconciliationResult.LeakageAmount.Currency, result.ReconciliationResult.LeakageAmount.MinorUnits),
		Cases:        cases,
	}
}

func newRunRows(runs []reconciliationservice.ReconciliationRunSummary) []ui.RunRow {
	rows := make([]ui.RunRow, 0, len(runs))
	for _, run := range runs {
		rows = append(rows, ui.RunRow{
			ID:           run.ID.String(),
			ContractID:   run.ContractID.String(),
			PeriodLabel:  ui.FormatPeriod(run.Period.Start, run.Period.End),
			Status:       ui.StatusLabel(string(run.Status)),
			StatusClass:  ui.StatusClass(string(run.Status)),
			StartedAt:    ui.FormatTime(run.StartedAt),
			CompletedAt:  ui.FormatTime(run.CompletedAt),
			Expected:     ui.FormatCount(run.ExpectedCount),
			Actual:       ui.FormatCount(run.ActualCount),
			Diff:         ui.FormatCount(run.DiffCount),
			Cases:        ui.FormatCount(run.CaseCount),
			LeakageLabel: ui.FormatMoney(run.LeakageAmount.Currency, run.LeakageAmount.MinorUnits),
			TraceID:      run.TraceID,
		})
	}

	return rows
}

func totalLeakageLabel(runs []reconciliationservice.ReconciliationRunSummary) string {
	if len(runs) == 0 {
		return ui.FormatMoney("USD", 0)
	}

	currency := runs[0].LeakageAmount.Currency
	var total int64
	for _, run := range runs {
		if run.LeakageAmount.Currency != currency {
			return "разные валюты"
		}

		total += run.LeakageAmount.MinorUnits
	}

	return ui.FormatMoney(currency, total)
}

func newCaseRows(tenantID uuid.UUID, cases []leakage.Case) []ui.CaseRow {
	return newCaseRowsFromReconciliation(tenantID.String(), cases)
}

func newCaseRowsFromReconciliation(tenantID string, cases []leakage.Case) []ui.CaseRow {
	rows := make([]ui.CaseRow, 0, len(cases))
	for _, item := range cases {
		assignee := item.Assignee
		if assignee == "" {
			assignee = "не назначен"
		}

		rows = append(rows, ui.CaseRow{
			ID:                item.ID.String(),
			TenantID:          item.TenantID.String(),
			ContractID:        item.ContractID.String(),
			RunID:             optionalUUIDString(item.ReconciliationRunID),
			Type:              ui.CaseTypeLabel(string(item.Type)),
			Severity:          ui.SeverityLabel(string(item.Severity)),
			SeverityValue:     string(item.Severity),
			SeverityClass:     ui.SeverityClass(string(item.Severity)),
			Status:            ui.StatusLabel(string(item.Status)),
			StatusValue:       string(item.Status),
			StatusClass:       ui.StatusClass(string(item.Status)),
			DetectedAt:        ui.FormatTime(item.DetectedAt),
			PeriodLabel:       ui.FormatPeriod(item.Period.Start, item.Period.End),
			ExpectedLabel:     ui.FormatMoney(item.ExpectedAmount.Currency, item.ExpectedAmount.MinorUnits),
			ActualLabel:       ui.FormatMoney(item.ActualAmount.Currency, item.ActualAmount.MinorUnits),
			LeakageLabel:      ui.FormatMoney(item.LeakageAmount.Currency, item.LeakageAmount.MinorUnits),
			LeakageMinorUnits: item.LeakageAmount.MinorUnits,
			Currency:          item.LeakageAmount.Currency,
			ConfidenceLabel:   ui.FormatConfidence(item.ConfidenceScore.BasisPoints),
			RootCause:         ui.RootCauseLabel(string(item.RootCauseCategory)),
			Assignee:          assignee,
			TraceID:           item.TraceID,
			DetailPath:        ui.CaseDetailPath(tenantID, item.ID.String()),
		})
	}

	return rows
}

func newCaseDetailPanelData(
	tenantID uuid.UUID,
	result caseapp.GetCaseResult,
) ui.CaseDetailPanelData {
	caseRows := newCaseRows(tenantID, []leakage.Case{result.Case})
	panel := ui.CaseDetailPanelData{
		TenantID:      tenantID.String(),
		Case:          caseRows[0],
		StatusOptions: ui.StatusOptions(string(result.Case.Status)),
		Evidence:      make([]ui.EvidenceRow, 0, len(result.Evidence)),
		RootCauses:    make([]ui.RootCauseRow, 0, len(result.RootCauses)),
		History:       make([]ui.HistoryRow, 0, len(result.History)),
	}

	for _, item := range result.Evidence {
		panel.Evidence = append(panel.Evidence, ui.EvidenceRow{
			ID:         item.ID.String(),
			Type:       string(item.Type),
			EntityType: item.EntityType,
			EntityID:   item.EntityID,
			Payload:    prettyJSON(item.Payload),
			CreatedAt:  ui.FormatTime(item.CreatedAt),
		})
	}

	for _, item := range result.RootCauses {
		panel.RootCauses = append(panel.RootCauses, ui.RootCauseRow{
			ID:              item.ID.String(),
			Category:        ui.RootCauseLabel(string(item.Category)),
			Subcategory:     item.Subcategory,
			Description:     item.Description,
			ConfidenceLabel: ui.FormatConfidence(item.ConfidenceScore.BasisPoints),
			DerivedBy:       ui.DerivedByLabel(string(item.DerivedBy)),
			CreatedAt:       ui.FormatTime(item.CreatedAt),
		})
	}

	for _, item := range result.History {
		panel.History = append(panel.History, ui.HistoryRow{
			ID:         item.ID.String(),
			FromStatus: ui.StatusLabel(string(item.FromStatus)),
			ToStatus:   ui.StatusLabel(string(item.ToStatus)),
			ChangedAt:  ui.FormatTime(item.ChangedAt),
			ChangedBy:  item.ChangedBy,
			ReasonCode: item.ReasonCode,
			Comment:    item.Comment,
		})
	}

	return panel
}

func prettyJSON(value map[string]any) string {
	if len(value) == 0 {
		return "{}"
	}

	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "{}"
	}

	return string(encoded)
}
