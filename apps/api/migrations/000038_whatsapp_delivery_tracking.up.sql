ALTER TABLE campaign_automation_runs
  ADD COLUMN IF NOT EXISTS provider_message_id varchar(180),
  ADD COLUMN IF NOT EXISTS delivered_at timestamptz,
  ADD COLUMN IF NOT EXISTS provider_status varchar(32),
  ADD COLUMN IF NOT EXISTS provider_payload jsonb NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS campaign_automation_runs_provider_idx ON campaign_automation_runs(provider_message_id) WHERE provider_message_id IS NOT NULL;
