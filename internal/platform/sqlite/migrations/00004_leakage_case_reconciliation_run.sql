-- +goose Up
ALTER TABLE leakage_cases ADD COLUMN reconciliation_run_id TEXT REFERENCES reconciliation_runs(id) ON DELETE SET NULL;

CREATE INDEX leakage_cases_reconciliation_run_idx
  ON leakage_cases(reconciliation_run_id);

-- +goose Down
DROP INDEX IF EXISTS leakage_cases_reconciliation_run_idx;

CREATE TABLE leakage_cases_tmp (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
  contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
  case_type TEXT NOT NULL,
  severity TEXT NOT NULL,
  status TEXT NOT NULL,
  detected_at TEXT NOT NULL,
  period_start TEXT NOT NULL,
  period_end TEXT NOT NULL,
  expected_amount_minor_units INTEGER NOT NULL,
  actual_amount_minor_units INTEGER NOT NULL,
  leakage_amount_minor_units INTEGER NOT NULL,
  currency TEXT NOT NULL,
  confidence_score_basis_points INTEGER NOT NULL,
  root_cause_category TEXT NOT NULL DEFAULT '',
  assignee TEXT NOT NULL DEFAULT '',
  trace_id TEXT NOT NULL DEFAULT ''
);

INSERT INTO leakage_cases_tmp (
  id,
  tenant_id,
  customer_id,
  contract_id,
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
)
SELECT
  id,
  tenant_id,
  customer_id,
  contract_id,
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
FROM leakage_cases;

DROP TABLE leakage_cases;
ALTER TABLE leakage_cases_tmp RENAME TO leakage_cases;

CREATE INDEX leakage_cases_contract_period_idx
  ON leakage_cases(tenant_id, contract_id, period_start, period_end);

CREATE INDEX leakage_cases_status_severity_idx
  ON leakage_cases(tenant_id, status, severity);
