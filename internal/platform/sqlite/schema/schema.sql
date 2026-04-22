CREATE TABLE tenants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    industry TEXT NOT NULL DEFAULT '',
    currency TEXT NOT NULL,
    timezone TEXT NOT NULL,
    settings_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE TABLE customer_accounts (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    external_id TEXT,
    name TEXT NOT NULL,
    segment TEXT NOT NULL DEFAULT '',
    billing_profile_id TEXT,
    status TEXT NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX customer_accounts_tenant_external_id_idx
    ON customer_accounts(tenant_id, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX customer_accounts_tenant_status_idx
    ON customer_accounts(tenant_id, status);

CREATE TABLE billable_items (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL,
    pricing_mode TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX billable_items_tenant_code_idx
    ON billable_items(tenant_id, code);

CREATE TABLE contracts (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
    external_id TEXT,
    status TEXT NOT NULL,
    start_date TEXT NOT NULL,
    end_date TEXT,
    currency TEXT NOT NULL,
    version INTEGER NOT NULL,
    billing_model TEXT NOT NULL,
    signed_at TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX contracts_tenant_external_id_idx
    ON contracts(tenant_id, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX contracts_tenant_customer_idx
    ON contracts(tenant_id, customer_id);

CREATE INDEX contracts_tenant_status_idx
    ON contracts(tenant_id, status);

CREATE TABLE contract_terms (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    term_type TEXT NOT NULL,
    effective_from TEXT NOT NULL,
    effective_to TEXT,
    priority INTEGER NOT NULL DEFAULT 0,
    expression_json TEXT NOT NULL,
    source_ref TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX contract_terms_contract_effective_idx
    ON contract_terms(contract_id, effective_from, effective_to);

CREATE INDEX contract_terms_tenant_type_idx
    ON contract_terms(tenant_id, term_type);

CREATE TABLE usage_records (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
    contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    billable_item_id TEXT NOT NULL REFERENCES billable_items(id) ON DELETE RESTRICT,
    external_id TEXT NOT NULL,
    usage_time TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    unit TEXT NOT NULL,
    source_system TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX usage_records_source_external_idx
    ON usage_records(tenant_id, source_system, external_id);

CREATE INDEX usage_records_contract_time_idx
    ON usage_records(tenant_id, contract_id, usage_time);

CREATE TABLE invoices (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
    contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    external_id TEXT NOT NULL,
    invoice_number TEXT NOT NULL,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    issued_at TEXT NOT NULL,
    due_at TEXT,
    currency TEXT NOT NULL,
    total_amount_minor_units INTEGER NOT NULL,
    status TEXT NOT NULL,
    source_system TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX invoices_source_external_idx
    ON invoices(tenant_id, source_system, external_id);

CREATE INDEX invoices_contract_period_idx
    ON invoices(tenant_id, contract_id, period_start, period_end);

CREATE TABLE invoice_lines (
    id TEXT PRIMARY KEY,
    invoice_id TEXT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    billable_item_id TEXT REFERENCES billable_items(id) ON DELETE RESTRICT,
    description TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    unit_price_minor_units INTEGER NOT NULL,
    discount_amount_minor_units INTEGER NOT NULL DEFAULT 0,
    tax_amount_minor_units INTEGER NOT NULL DEFAULT 0,
    line_total_minor_units INTEGER NOT NULL,
    currency TEXT NOT NULL,
    source_ref TEXT NOT NULL DEFAULT '',
    pricing_snapshot_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE INDEX invoice_lines_invoice_idx
    ON invoice_lines(invoice_id);

CREATE INDEX invoice_lines_billable_item_idx
    ON invoice_lines(tenant_id, billable_item_id);

CREATE TABLE payments (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
    external_id TEXT NOT NULL,
    amount_minor_units INTEGER NOT NULL,
    currency TEXT NOT NULL,
    received_at TEXT NOT NULL,
    status TEXT NOT NULL,
    allocation_status TEXT NOT NULL,
    source_system TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX payments_source_external_idx
    ON payments(tenant_id, source_system, external_id);

CREATE INDEX payments_customer_received_idx
    ON payments(tenant_id, customer_id, received_at);

CREATE TABLE payment_allocations (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    payment_id TEXT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    invoice_id TEXT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    allocated_amount_minor_units INTEGER NOT NULL,
    currency TEXT NOT NULL,
    allocated_at TEXT NOT NULL
);

CREATE INDEX payment_allocations_payment_idx
    ON payment_allocations(payment_id);

CREATE INDEX payment_allocations_invoice_idx
    ON payment_allocations(invoice_id);

CREATE TABLE adjustments (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
    contract_id TEXT REFERENCES contracts(id) ON DELETE RESTRICT,
    adjustment_type TEXT NOT NULL,
    amount_minor_units INTEGER NOT NULL,
    currency TEXT NOT NULL,
    reason_code TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    source_ref TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX adjustments_contract_created_idx
    ON adjustments(tenant_id, contract_id, created_at);

CREATE TABLE sla_events (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
    contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    event_type TEXT NOT NULL,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    severity TEXT NOT NULL,
    impact_scope TEXT NOT NULL DEFAULT '',
    source_ref TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX sla_events_contract_started_idx
    ON sla_events(tenant_id, contract_id, started_at);

CREATE TABLE expected_revenue_entries (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
    contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    billable_item_id TEXT NOT NULL REFERENCES billable_items(id) ON DELETE RESTRICT,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    expected_amount_minor_units INTEGER NOT NULL,
    currency TEXT NOT NULL,
    calculation_basis_json TEXT NOT NULL DEFAULT '{}',
    calculated_at TEXT NOT NULL,
    version INTEGER NOT NULL,
    trace_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX expected_revenue_contract_period_idx
    ON expected_revenue_entries(tenant_id, contract_id, period_start, period_end);

CREATE INDEX expected_revenue_customer_period_idx
    ON expected_revenue_entries(tenant_id, customer_id, period_start, period_end);

CREATE TABLE actual_revenue_entries (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES customer_accounts(id) ON DELETE RESTRICT,
    contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    billable_item_id TEXT NOT NULL REFERENCES billable_items(id) ON DELETE RESTRICT,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    actual_amount_minor_units INTEGER NOT NULL,
    currency TEXT NOT NULL,
    recognized_from TEXT NOT NULL,
    recognized_at TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX actual_revenue_contract_period_idx
    ON actual_revenue_entries(tenant_id, contract_id, period_start, period_end);

CREATE INDEX actual_revenue_customer_period_idx
    ON actual_revenue_entries(tenant_id, customer_id, period_start, period_end);

CREATE TABLE reconciliation_runs (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    expected_count INTEGER NOT NULL DEFAULT 0,
    actual_count INTEGER NOT NULL DEFAULT 0,
    diff_count INTEGER NOT NULL DEFAULT 0,
    case_count INTEGER NOT NULL DEFAULT 0,
    leakage_amount_minor_units INTEGER NOT NULL DEFAULT 0,
    currency TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX reconciliation_runs_contract_period_idx
    ON reconciliation_runs(tenant_id, contract_id, period_start, period_end);

CREATE TABLE leakage_cases (
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

CREATE INDEX leakage_cases_contract_period_idx
    ON leakage_cases(tenant_id, contract_id, period_start, period_end);

CREATE INDEX leakage_cases_status_severity_idx
    ON leakage_cases(tenant_id, status, severity);

CREATE TABLE leakage_evidence (
    id TEXT PRIMARY KEY,
    case_id TEXT NOT NULL REFERENCES leakage_cases(id) ON DELETE CASCADE,
    evidence_type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE INDEX leakage_evidence_case_idx
    ON leakage_evidence(case_id);

CREATE TABLE root_causes (
    id TEXT PRIMARY KEY,
    case_id TEXT NOT NULL REFERENCES leakage_cases(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    subcategory TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL,
    confidence_score_basis_points INTEGER NOT NULL,
    derived_by TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX root_causes_case_idx
    ON root_causes(case_id);

CREATE TABLE outbox_events (
    id TEXT PRIMARY KEY,
    tenant_id TEXT REFERENCES tenants(id) ON DELETE CASCADE,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    message_key TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    headers_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    published_at TEXT
);

CREATE INDEX outbox_events_status_occurred_idx
    ON outbox_events(status, occurred_at);
