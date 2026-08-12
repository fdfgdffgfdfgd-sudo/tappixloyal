DROP INDEX IF EXISTS report_runs_queue_idx;
ALTER TABLE report_runs DROP COLUMN IF EXISTS next_attempt_at;
CREATE INDEX report_runs_queue_idx ON report_runs(status,created_at)
  WHERE status IN ('queued','processing');
