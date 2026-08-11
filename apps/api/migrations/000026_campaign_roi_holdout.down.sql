DELETE FROM role_permissions WHERE permission='campaigns.analytics';
DROP INDEX IF EXISTS campaign_recipients_attribution_idx;
DROP INDEX IF EXISTS campaign_recipients_experiment_idx;
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS clicked_at,DROP COLUMN IF EXISTS opened_at,DROP COLUMN IF EXISTS delivered_at,DROP COLUMN IF EXISTS experiment_group;
ALTER TABLE marketing_campaigns DROP COLUMN IF EXISTS holdout_seed,DROP COLUMN IF EXISTS holdout_percent,DROP COLUMN IF EXISTS attribution_window_days,DROP COLUMN IF EXISTS reward_cost,DROP COLUMN IF EXISTS message_cost;
