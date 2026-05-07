-- name: CreateOutboxEvent :exec
INSERT INTO outbox_events (
    id,
    topic,
    event_type,
    tenant_id,
    contract_id,
    partition_key,
    payload_json,
    headers_json,
    status,
    attempts,
    last_error,
    available_at,
    created_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(topic),
    sqlc.arg(event_type),
    sqlc.arg(tenant_id),
    sqlc.narg(contract_id),
    sqlc.arg(partition_key),
    sqlc.arg(payload_json),
    sqlc.arg(headers_json),
    sqlc.arg(status),
    0,
    '',
    sqlc.arg(available_at),
    sqlc.arg(created_at)
);

-- name: ListPendingOutboxEvents :many
SELECT
    id,
    topic,
    event_type,
    tenant_id,
    contract_id,
    partition_key,
    payload_json,
    headers_json,
    status,
    attempts,
    last_error,
    available_at,
    created_at,
    published_at
FROM outbox_events
WHERE status IN ('pending', 'failed')
  AND available_at <= sqlc.arg(available_at)
ORDER BY created_at ASC
LIMIT sqlc.arg(limit_count);

-- name: MarkOutboxEventPublished :exec
UPDATE outbox_events
SET
    status = 'published',
    published_at = sqlc.arg(published_at),
    last_error = ''
WHERE id = sqlc.arg(id);

-- name: MarkOutboxEventFailed :exec
UPDATE outbox_events
SET
    status = 'failed',
    attempts = attempts + 1,
    last_error = sqlc.arg(last_error),
    available_at = sqlc.arg(available_at)
WHERE id = sqlc.arg(id);

-- name: GetInboxEvent :one
SELECT
    event_id,
    handler,
    topic,
    source_key,
    processed_at
FROM inbox_events
WHERE event_id = sqlc.arg(event_id)
  AND handler = sqlc.arg(handler);

-- name: CreateInboxEvent :exec
INSERT OR IGNORE INTO inbox_events (
    event_id,
    handler,
    topic,
    source_key,
    processed_at
) VALUES (
    sqlc.arg(event_id),
    sqlc.arg(handler),
    sqlc.arg(topic),
    sqlc.arg(source_key),
    sqlc.arg(processed_at)
);
