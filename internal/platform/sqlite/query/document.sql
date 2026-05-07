-- name: CreateDocument :exec
INSERT INTO documents (
    id,
    tenant_id,
    source_type,
    file_name,
    content_type,
    size_bytes,
    sha256_hash,
    storage_key,
    status,
    uploaded_at,
    metadata_json
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
)
ON CONFLICT(tenant_id, sha256_hash) DO UPDATE SET
    source_type = excluded.source_type,
    file_name = excluded.file_name,
    content_type = excluded.content_type,
    size_bytes = excluded.size_bytes,
    storage_key = excluded.storage_key,
    status = excluded.status,
    uploaded_at = excluded.uploaded_at,
    metadata_json = excluded.metadata_json;

-- name: GetDocument :one
SELECT *
FROM documents
WHERE tenant_id = ? AND id = ?;

-- name: ListDocuments :many
SELECT *
FROM documents
WHERE tenant_id = ?
ORDER BY uploaded_at DESC
LIMIT ? OFFSET ?;

-- name: CreateDocumentExtractionDraft :exec
INSERT INTO document_extraction_drafts (
    id,
    document_id,
    tenant_id,
    draft_type,
    status,
    model,
    prompt_version,
    input_hash,
    output_json,
    evidence_json,
    confidence_score_basis_points,
    created_at,
    reviewed_by,
    reviewed_at,
    rejection_reason
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
);

-- name: ListDocumentExtractionDrafts :many
SELECT *
FROM document_extraction_drafts
WHERE tenant_id = ? AND document_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetDocumentExtractionDraft :one
SELECT *
FROM document_extraction_drafts
WHERE tenant_id = ? AND id = ?;
