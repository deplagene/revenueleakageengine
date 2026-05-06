-- +goose Up
DROP INDEX IF EXISTS outbox_events_status_occurred_idx;

CREATE TABLE outbox_events_new (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    event_type TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    contract_id TEXT,
    partition_key TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    headers_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    available_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    published_at TEXT
);

INSERT INTO outbox_events_new (
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
)
SELECT
    id,
    topic,
    aggregate_type,
    COALESCE(tenant_id, ''),
    NULL,
    message_key,
    payload_json,
    headers_json,
    status,
    0,
    '',
    occurred_at,
    occurred_at,
    published_at
FROM outbox_events;

DROP TABLE outbox_events;
ALTER TABLE outbox_events_new RENAME TO outbox_events;

CREATE INDEX outbox_events_pending_idx
    ON outbox_events(status, available_at, created_at);

CREATE INDEX outbox_events_tenant_created_idx
    ON outbox_events(tenant_id, created_at);

CREATE TABLE inbox_events (
    event_id TEXT NOT NULL,
    handler TEXT NOT NULL,
    topic TEXT NOT NULL,
    source_key TEXT NOT NULL DEFAULT '',
    processed_at TEXT NOT NULL,
    PRIMARY KEY (event_id, handler)
);

CREATE INDEX inbox_events_topic_processed_idx
    ON inbox_events(topic, processed_at);

-- +goose Down
DROP INDEX IF EXISTS inbox_events_topic_processed_idx;
DROP TABLE IF EXISTS inbox_events;
DROP INDEX IF EXISTS outbox_events_tenant_created_idx;
DROP INDEX IF EXISTS outbox_events_pending_idx;

CREATE TABLE outbox_events_old (
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

INSERT INTO outbox_events_old (
    id,
    tenant_id,
    aggregate_type,
    aggregate_id,
    topic,
    message_key,
    payload_json,
    headers_json,
    status,
    occurred_at,
    published_at
)
SELECT
    id,
    NULLIF(tenant_id, ''),
    event_type,
    id,
    topic,
    partition_key,
    payload_json,
    headers_json,
    status,
    created_at,
    published_at
FROM outbox_events;

DROP TABLE outbox_events;
ALTER TABLE outbox_events_old RENAME TO outbox_events;

CREATE INDEX outbox_events_status_occurred_idx
    ON outbox_events(status, occurred_at);
