-- +goose Up
CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    sha256_hash TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    status TEXT NOT NULL,
    uploaded_at TEXT NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE UNIQUE INDEX documents_tenant_hash_idx
    ON documents(tenant_id, sha256_hash);

CREATE INDEX documents_tenant_uploaded_idx
    ON documents(tenant_id, uploaded_at);

CREATE TABLE document_extraction_drafts (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    draft_type TEXT NOT NULL,
    status TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    output_json TEXT NOT NULL,
    evidence_json TEXT NOT NULL DEFAULT '[]',
    confidence_score_basis_points INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    reviewed_by TEXT NOT NULL DEFAULT '',
    reviewed_at TEXT,
    rejection_reason TEXT NOT NULL DEFAULT ''
);

CREATE INDEX document_extraction_drafts_document_idx
    ON document_extraction_drafts(document_id, created_at);

CREATE INDEX document_extraction_drafts_tenant_status_idx
    ON document_extraction_drafts(tenant_id, status, created_at);

-- +goose Down
DROP TABLE IF EXISTS document_extraction_drafts;
DROP TABLE IF EXISTS documents;
