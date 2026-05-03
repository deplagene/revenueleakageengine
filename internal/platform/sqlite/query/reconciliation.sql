-- name: ListExpectedRevenueByContractPeriod :many
SELECT
    id,
    tenant_id,
    customer_id,
    contract_id,
    billable_item_id,
    period_start,
    period_end,
    expected_amount_minor_units,
    currency,
    calculation_basis_json,
    calculated_at,
    version,
    trace_id
FROM expected_revenue_entries
WHERE tenant_id = sqlc.arg(tenant_id)
  AND contract_id = sqlc.arg(contract_id)
  AND period_start >= sqlc.arg(period_start)
  AND period_end <= sqlc.arg(period_end)
ORDER BY customer_id, billable_item_id, period_start, period_end, calculated_at;

-- name: ListActualRevenueByContractPeriod :many
SELECT
    id,
    tenant_id,
    customer_id,
    contract_id,
    billable_item_id,
    period_start,
    period_end,
    actual_amount_minor_units,
    currency,
    recognized_from,
    recognized_at,
    trace_id
FROM actual_revenue_entries
WHERE tenant_id = sqlc.arg(tenant_id)
  AND contract_id = sqlc.arg(contract_id)
  AND period_start >= sqlc.arg(period_start)
  AND period_end <= sqlc.arg(period_end)
ORDER BY customer_id, billable_item_id, period_start, period_end, recognized_at;

-- name: ListReconciliationRuns :many
SELECT
    id,
    tenant_id,
    contract_id,
    period_start,
    period_end,
    status,
    started_at,
    completed_at,
    expected_count,
    actual_count,
    diff_count,
    case_count,
    leakage_amount_minor_units,
    currency,
    trace_id
FROM reconciliation_runs
WHERE (sqlc.narg(tenant_id) IS NULL OR tenant_id = sqlc.narg(tenant_id))
  AND (sqlc.narg(contract_id) IS NULL OR contract_id = sqlc.narg(contract_id))
ORDER BY started_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: GetReconciliationRun :one
SELECT
    id,
    tenant_id,
    contract_id,
    period_start,
    period_end,
    status,
    started_at,
    completed_at,
    expected_count,
    actual_count,
    diff_count,
    case_count,
    leakage_amount_minor_units,
    currency,
    trace_id
FROM reconciliation_runs
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: CreateReconciliationRun :exec
INSERT INTO reconciliation_runs (
    id,
    tenant_id,
    contract_id,
    period_start,
    period_end,
    status,
    started_at,
    expected_count,
    actual_count,
    diff_count,
    case_count,
    leakage_amount_minor_units,
    currency,
    trace_id
) VALUES (
    sqlc.arg(id),
    sqlc.arg(tenant_id),
    sqlc.arg(contract_id),
    sqlc.arg(period_start),
    sqlc.arg(period_end),
    sqlc.arg(status),
    sqlc.arg(started_at),
    sqlc.arg(expected_count),
    sqlc.arg(actual_count),
    sqlc.arg(diff_count),
    sqlc.arg(case_count),
    sqlc.arg(leakage_amount_minor_units),
    sqlc.arg(currency),
    sqlc.arg(trace_id)
);

-- name: CompleteReconciliationRun :exec
UPDATE reconciliation_runs
SET
    status = sqlc.arg(status),
    completed_at = sqlc.arg(completed_at),
    expected_count = sqlc.arg(expected_count),
    actual_count = sqlc.arg(actual_count),
    diff_count = sqlc.arg(diff_count),
    case_count = sqlc.arg(case_count),
    leakage_amount_minor_units = sqlc.arg(leakage_amount_minor_units)
WHERE id = sqlc.arg(id);

-- name: CreateLeakageCase :exec
INSERT INTO leakage_cases (
    id,
    tenant_id,
    customer_id,
    contract_id,
    reconciliation_run_id,
    case_type,
    severity,
    status,
    detected_at,
    period_start,
    period_end,
    expected_amount_minor_units,
    actual_amount_minor_units,
    leakage_amount_minor_units,
    currency,
    confidence_score_basis_points,
    root_cause_category,
    assignee,
    trace_id
) VALUES (
    sqlc.arg(id),
    sqlc.arg(tenant_id),
    sqlc.arg(customer_id),
    sqlc.arg(contract_id),
    sqlc.narg(reconciliation_run_id),
    sqlc.arg(case_type),
    sqlc.arg(severity),
    sqlc.arg(status),
    sqlc.arg(detected_at),
    sqlc.arg(period_start),
    sqlc.arg(period_end),
    sqlc.arg(expected_amount_minor_units),
    sqlc.arg(actual_amount_minor_units),
    sqlc.arg(leakage_amount_minor_units),
    sqlc.arg(currency),
    sqlc.arg(confidence_score_basis_points),
    sqlc.arg(root_cause_category),
    sqlc.arg(assignee),
    sqlc.arg(trace_id)
);

-- name: CreateLeakageEvidence :exec
INSERT INTO leakage_evidence (
    id,
    case_id,
    evidence_type,
    entity_type,
    entity_id,
    payload_json,
    created_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(case_id),
    sqlc.arg(evidence_type),
    sqlc.arg(entity_type),
    sqlc.arg(entity_id),
    sqlc.arg(payload_json),
    sqlc.arg(created_at)
);
