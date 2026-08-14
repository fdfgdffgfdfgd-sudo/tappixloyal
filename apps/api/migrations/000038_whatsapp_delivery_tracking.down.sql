DROP INDEX IF EXISTS campaign_automation_runs_provider_idx;
ALTER TABLE campaign_automation_runs DROP COLUMN IF EXISTS provider_payload,DROP COLUMN IF EXISTS provider_status,DROP COLUMN IF EXISTS delivered_at,DROP COLUMN IF EXISTS provider_message_id;
