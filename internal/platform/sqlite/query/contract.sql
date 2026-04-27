-- name: GetContract :one
SELECT
  *
FROM
  contracts
WHERE
  id = ?
LIMIT
  1;

-- name: ListContractsByCustomer :many
SELECT
  *
FROM
  contracts
WHERE
  tenant_id = ?
  AND customer_id = ?
ORDER BY
  created_at DESC;

-- name: UpsertContract :exec
INSERT INTO
  contracts (
    id,
    tenant_id,
    customer_id,
    external_id,
    status,
    start_date,
    end_date,
    currency,
    version,
    billing_model,
    signed_at,
    metadata_json,
    created_at,
    updated_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE
SET
  status = excluded.status,
  start_date = excluded.start_date,
  end_date = excluded.end_date,
  version = excluded.version,
  billing_model = excluded.billing_model,
  signed_at = excluded.signed_at,
  metadata_json = excluded.metadata_json,
  updated_at = excluded.updated_at;

-- name: GetBillableItem :one
SELECT
  *
FROM
  billable_items
WHERE
  id = ?
LIMIT
  1;

-- name: GetBillableItemByCode :one
SELECT
  *
FROM
  billable_items
WHERE
  tenant_id = ?
  AND code = ?
LIMIT
  1;

-- name: ListBillableItems :many
SELECT
  *
FROM
  billable_items
WHERE
  tenant_id = ?
ORDER BY
  code;

-- name: UpsertBillableItem :exec
INSERT INTO
  billable_items (
    id,
    tenant_id,
    code,
    name,
    category,
    unit,
    pricing_mode,
    status,
    created_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE
SET
  name = excluded.name,
  category = excluded.category,
  unit = excluded.unit,
  pricing_mode = excluded.pricing_mode,
  status = excluded.status;

-- name: ListContractTerms :many
SELECT
  *
FROM
  contract_terms
WHERE
  contract_id = ?
ORDER BY
  priority DESC,
  created_at DESC;

-- name: ListEffectiveContractTerms :many
SELECT
  *
FROM
  contract_terms
WHERE
  contract_id = ?
  AND effective_from <= ?
  AND (
    effective_to IS NULL
    OR effective_to >= ?
  )
ORDER BY
  priority DESC,
  created_at DESC;

-- name: UpsertContractTerm :exec
INSERT INTO
  contract_terms (
    id,
    tenant_id,
    contract_id,
    term_type,
    effective_from,
    effective_to,
    priority,
    expression_json,
    source_ref,
    created_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE
SET
  term_type = excluded.term_type,
  effective_from = excluded.effective_from,
  effective_to = excluded.effective_to,
  priority = excluded.priority,
  expression_json = excluded.expression_json,
  source_ref = excluded.source_ref;
