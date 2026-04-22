-- name: CreateExpectedRevenueEntry :exec
INSERT INTO expected_revenue_entries (
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
) VALUES (
    sqlc.arg(id),
    sqlc.arg(tenant_id),
    sqlc.arg(customer_id),
    sqlc.arg(contract_id),
    sqlc.arg(billable_item_id),
    sqlc.arg(period_start),
    sqlc.arg(period_end),
    sqlc.arg(expected_amount_minor_units),
    sqlc.arg(currency),
    sqlc.arg(calculation_basis_json),
    sqlc.arg(calculated_at),
    sqlc.arg(version),
    sqlc.arg(trace_id)
);

-- name: CreateActualRevenueEntry :exec
INSERT INTO actual_revenue_entries (
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
) VALUES (
    sqlc.arg(id),
    sqlc.arg(tenant_id),
    sqlc.arg(customer_id),
    sqlc.arg(contract_id),
    sqlc.arg(billable_item_id),
    sqlc.arg(period_start),
    sqlc.arg(period_end),
    sqlc.arg(actual_amount_minor_units),
    sqlc.arg(currency),
    sqlc.arg(recognized_from),
    sqlc.arg(recognized_at),
    sqlc.arg(trace_id)
);
