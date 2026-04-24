-- +goose Up
ALTER TABLE case_status_history ADD COLUMN reason_code TEXT NOT NULL DEFAULT '';
ALTER TABLE case_status_history ADD COLUMN comment TEXT NOT NULL DEFAULT '';

-- +goose Down
CREATE TABLE case_status_history_tmp (
  id TEXT PRIMARY KEY,
  case_id TEXT NOT NULL REFERENCES leakage_cases (id) ON DELETE CASCADE,
  from_status TEXT NOT NULL,
  to_status TEXT NOT NULL,
  changed_at TEXT NOT NULL,
  changed_by TEXT NOT NULL DEFAULT ''
);

INSERT INTO case_status_history_tmp (id, case_id, from_status, to_status, changed_at, changed_by)
SELECT id, case_id, from_status, to_status, changed_at, changed_by
FROM case_status_history;

DROP TABLE case_status_history;
ALTER TABLE case_status_history_tmp RENAME TO case_status_history;
CREATE INDEX case_status_history_case_changed_idx ON case_status_history (case_id, changed_at);
