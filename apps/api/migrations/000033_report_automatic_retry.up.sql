ALTER TABLE report_runs
  ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now();

DROP INDEX IF EXISTS report_runs_queue_idx;
CREATE INDEX report_runs_queue_idx ON report_runs(status,next_attempt_at,created_at)
  WHERE status IN ('queued','processing');
