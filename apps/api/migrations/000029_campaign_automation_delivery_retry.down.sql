DROP INDEX IF EXISTS campaign_automation_runs_retry_idx;
ALTER TABLE campaign_automation_runs DROP CONSTRAINT IF EXISTS campaign_automation_runs_attempt_count_check;
ALTER TABLE campaign_automation_runs
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS next_attempt_at,
  DROP COLUMN IF EXISTS attempt_count;
