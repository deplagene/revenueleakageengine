-- name: UpsertUsageRecord :exec
INSERT INTO
  usage_records (
    id,
    tenant_id,
    customer_id,
    contract_id,
    billable_item_id,
    external_id,
    usage_time,
    quantity,
    unit,
    source_system,
    trace_id,
    metadata_json,
    created_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (tenant_id, source_system, external_id) DO UPDATE
SET
  customer_id = excluded.customer_id,
  contract_id = excluded.contract_id,
  billable_item_id = excluded.billable_item_id,
  usage_time = excluded.usage_time,
  quantity = excluded.quantity,
  unit = excluded.unit,
  trace_id = excluded.trace_id,
  metadata_json = excluded.metadata_json;

-- name: UpsertInvoice :exec
INSERT INTO
  invoices (
    id,
    tenant_id,
    customer_id,
    contract_id,
    external_id,
    invoice_number,
    period_start,
    period_end,
    issued_at,
    due_at,
    currency,
    total_amount_minor_units,
    status,
    source_system,
    created_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (tenant_id, source_system, external_id) DO UPDATE
SET
  customer_id = excluded.customer_id,
  contract_id = excluded.contract_id,
  invoice_number = excluded.invoice_number,
  period_start = excluded.period_start,
  period_end = excluded.period_end,
  issued_at = excluded.issued_at,
  due_at = excluded.due_at,
  currency = excluded.currency,
  total_amount_minor_units = excluded.total_amount_minor_units,
  status = excluded.status;

-- name: GetInvoiceBySourceExternal :one
SELECT
  *
FROM
  invoices
WHERE
  tenant_id = ?
  AND source_system = ?
  AND external_id = ?
LIMIT
  1;

-- name: DeleteInvoiceLinesByInvoice :exec
DELETE FROM invoice_lines
WHERE
  invoice_id = ?;

-- name: CreateInvoiceLine :exec
INSERT INTO
  invoice_lines (
    id,
    invoice_id,
    tenant_id,
    billable_item_id,
    description,
    quantity,
    unit_price_minor_units,
    discount_amount_minor_units,
    tax_amount_minor_units,
    line_total_minor_units,
    currency,
    source_ref,
    pricing_snapshot_json,
    created_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
