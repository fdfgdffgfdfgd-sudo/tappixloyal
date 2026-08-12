ALTER TABLE campaign_automation_runs
  ADD COLUMN IF NOT EXISTS attempt_count integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS next_attempt_at timestamptz,
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE campaign_automation_runs
  ADD CONSTRAINT campaign_automation_runs_attempt_count_check
  CHECK(attempt_count BETWEEN 0 AND 10) NOT VALID;
ALTER TABLE campaign_automation_runs VALIDATE CONSTRAINT campaign_automation_runs_attempt_count_check;

-- Runs left by an interrupted worker are reclaimable on its next pass.
UPDATE campaign_automation_runs SET updated_at=created_at WHERE status='pending';

CREATE INDEX IF NOT EXISTS campaign_automation_runs_retry_idx
  ON campaign_automation_runs(status,next_attempt_at,updated_at)
  WHERE status IN ('pending','failed');
