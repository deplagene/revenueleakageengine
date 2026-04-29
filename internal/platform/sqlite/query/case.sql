-- name: ListLeakageCases :many
SELECT
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
FROM leakage_cases
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (sqlc.narg(contract_id) IS NULL OR contract_id = sqlc.narg(contract_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
ORDER BY detected_at DESC, id DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: GetLeakageCase :one
SELECT
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
FROM leakage_cases
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(case_id);

-- name: ListLeakageEvidenceByCase :many
SELECT
    id,
    case_id,
    evidence_type,
    entity_type,
    entity_id,
    payload_json,
    created_at
FROM leakage_evidence
WHERE case_id = sqlc.arg(case_id)
ORDER BY created_at ASC, id ASC;

-- name: ListRootCausesByCase :many
SELECT
    id,
    case_id,
    category,
    subcategory,
    description,
    confidence_score_basis_points,
    derived_by,
    created_at
FROM root_causes
WHERE case_id = sqlc.arg(case_id)
ORDER BY created_at ASC, id ASC;

-- name: UpdateLeakageCaseStatus :execrows
UPDATE leakage_cases
SET status = sqlc.arg(status)
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(case_id);

-- name: UpdateLeakageCaseAssignee :execrows
UPDATE leakage_cases
SET assignee = sqlc.arg(assignee)
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(case_id);

-- name: CreateCaseStatusHistory :exec
INSERT INTO case_status_history (
    id,
    case_id,
    from_status,
    to_status,
    changed_at,
    changed_by,
    reason_code,
    comment
) VALUES (
    sqlc.arg(id),
    sqlc.arg(case_id),
    sqlc.arg(from_status),
    sqlc.arg(to_status),
    sqlc.arg(changed_at),
    sqlc.arg(changed_by),
    sqlc.arg(reason_code),
    sqlc.arg(comment)
);

-- name: ListCaseStatusHistoryByCase :many
SELECT
    id,
    case_id,
    from_status,
    to_status,
    changed_at,
    changed_by,
    reason_code,
    comment
FROM case_status_history
WHERE case_id = sqlc.arg(case_id)
ORDER BY changed_at ASC, id ASC;
