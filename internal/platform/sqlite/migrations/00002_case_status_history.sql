-- +goose Up
CREATE TABLE case_status_history (
  id TEXT PRIMARY KEY,
  case_id TEXT NOT NULL REFERENCES leakage_cases (id) ON DELETE CASCADE,
  from_status TEXT NOT NULL,
  to_status TEXT NOT NULL,
  changed_at TEXT NOT NULL,
  changed_by TEXT NOT NULL DEFAULT ''
);

CREATE INDEX case_status_history_case_changed_idx ON case_status_history (case_id, changed_at);

-- +goose Down
DROP INDEX IF EXISTS case_status_history_case_changed_idx;

DROP TABLE IF EXISTS case_status_history;
