package ui

import (
	"fmt"
	"strings"
	"time"
)

type DashboardPageData struct {
	Title              string
	TenantID           string
	Error              string
	TotalLeakageLabel  string
	CompletedRunsLabel string
	OpenCasesLabel     string
	RunForm            RunFormData
	Runs               []RunRow
	RecentCases        CasesTableData
}

type RunFormData struct {
	TenantID                 string
	ContractID               string
	PeriodStart              string
	PeriodEnd                string
	Currency                 string
	MinimumLeakageMinorUnits string
	TraceID                  string
	Error                    string
}

type RunResultData struct {
	RunID        string
	ExpectedID   string
	ActualCount  string
	DiffCount    string
	CaseCount    string
	LeakageLabel string
	Cases        []CaseRow
	Error        string
}

type RunsTableData struct {
	Runs []RunRow
}

type RunRow struct {
	ID           string
	ContractID   string
	PeriodLabel  string
	Status       string
	StatusClass  string
	StartedAt    string
	CompletedAt  string
	Expected     string
	Actual       string
	Diff         string
	Cases        string
	LeakageLabel string
	TraceID      string
}

type CasesPageData struct {
	Title string
	Table CasesTableData
}

type CasesTableData struct {
	TenantID     string
	ContractID   string
	Status       string
	Error        string
	EmptyMessage string
	Cases        []CaseRow
}

type CaseDetailPageData struct {
	Title string
	Panel CaseDetailPanelData
}

type CaseDetailPanelData struct {
	TenantID      string
	Case          CaseRow
	Evidence      []EvidenceRow
	RootCauses    []RootCauseRow
	History       []HistoryRow
	Error         string
	StatusOptions []StatusOption
}

type CaseRow struct {
	ID              string
	TenantID        string
	ContractID      string
	RunID           string
	Type            string
	Severity        string
	Status          string
	StatusClass     string
	DetectedAt      string
	PeriodLabel     string
	ExpectedLabel   string
	ActualLabel     string
	LeakageLabel    string
	ConfidenceLabel string
	RootCause       string
	Assignee        string
	TraceID         string
	DetailPath      string
}

type EvidenceRow struct {
	ID         string
	Type       string
	EntityType string
	EntityID   string
	Payload    string
	CreatedAt  string
}

type RootCauseRow struct {
	ID              string
	Category        string
	Subcategory     string
	Description     string
	ConfidenceLabel string
	DerivedBy       string
	CreatedAt       string
}

type HistoryRow struct {
	ID         string
	FromStatus string
	ToStatus   string
	ChangedAt  string
	ChangedBy  string
	ReasonCode string
	Comment    string
}

type StatusOption struct {
	Value    string
	Label    string
	Selected bool
}

func FormatMoney(currency string, minorUnits int64) string {
	if currency == "" {
		currency = "USD"
	}

	sign := ""
	if minorUnits < 0 {
		sign = "-"
		minorUnits = -minorUnits
	}

	return fmt.Sprintf("%s%s %d.%02d", sign, currency, minorUnits/100, minorUnits%100)
}

func FormatCount(value int64) string {
	return fmt.Sprintf("%d", value)
}

func FormatConfidence(basisPoints uint16) string {
	return fmt.Sprintf("%.2f%%", float64(basisPoints)/100)
}

func FormatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}

	return value.UTC().Format("2006-01-02 15:04 UTC")
}

func FormatPeriod(start, end time.Time) string {
	return fmt.Sprintf("%s -> %s", start.UTC().Format("2006-01-02"), end.UTC().Format("2006-01-02"))
}

func StatusClass(status string) string {
	return "status status-" + strings.ReplaceAll(strings.ToLower(status), "_", "-")
}

func StatusLabel(status string) string {
	switch status {
	case "open":
		return "Открыт"
	case "investigating":
		return "В работе"
	case "resolved":
		return "Решён"
	case "dismissed":
		return "Отклонён"
	case "running":
		return "Выполняется"
	case "completed":
		return "Завершён"
	default:
		return status
	}
}

func SeverityLabel(severity string) string {
	switch severity {
	case "low":
		return "Низкая"
	case "medium":
		return "Средняя"
	case "high":
		return "Высокая"
	case "critical":
		return "Критическая"
	default:
		return severity
	}
}

func CaseTypeLabel(caseType string) string {
	switch caseType {
	case "underbilling":
		return "Недовыставление"
	case "unbilled_usage":
		return "Невыставленное потребление"
	case "expired_discount_still_applied":
		return "Истёкшая скидка"
	case "contract_billing_mismatch":
		return "Расхождение контракта и биллинга"
	case "missing_invoice":
		return "Нет счёта"
	case "payment_shortfall":
		return "Недоплата"
	case "unclaimed_rebate":
		return "Неприменённый rebate"
	case "over_crediting":
		return "Избыточный кредит"
	case "sla_credit_not_accounted":
		return "SLA-кредит не учтён"
	case "pricing_rule_misapplied":
		return "Ошибка правила тарификации"
	default:
		return caseType
	}
}

func RootCauseLabel(category string) string {
	switch category {
	case "":
		return "Не определена"
	case "contract_config_error":
		return "Ошибка настройки контракта"
	case "billing_engine_bug":
		return "Ошибка биллингового движка"
	case "manual_process_failure":
		return "Сбой ручного процесса"
	case "crm_billing_sync_gap":
		return "Разрыв CRM и биллинга"
	case "payment_allocation_issue":
		return "Проблема аллокации платежа"
	case "unauthorized_discount":
		return "Несанкционированная скидка"
	case "missing_usage_ingestion":
		return "Не загружено потребление"
	case "sla_compensation_logic_gap":
		return "Пробел в SLA-компенсации"
	default:
		return category
	}
}

func DerivedByLabel(value string) string {
	switch value {
	case "rules":
		return "правила"
	case "ai":
		return "ИИ"
	case "human":
		return "оператор"
	default:
		return value
	}
}

func navClass(active, item string) string {
	if active == item {
		return "active"
	}

	return ""
}

func CaseDetailPath(tenantID, caseID string) string {
	return fmt.Sprintf("/ui/cases/%s?tenant_id=%s", caseID, tenantID)
}

func StatusOptions(current string) []StatusOption {
	values := []StatusOption{
		{Value: "open", Label: "Открыт"},
		{Value: "investigating", Label: "В работе"},
		{Value: "resolved", Label: "Решить"},
		{Value: "dismissed", Label: "Отклонить"},
	}

	for i := range values {
		values[i].Selected = values[i].Value == current
	}

	return values
}
