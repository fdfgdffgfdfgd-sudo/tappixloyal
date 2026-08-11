ALTER TABLE marketing_campaigns
  ADD COLUMN message_cost numeric(12,4) NOT NULL DEFAULT 0 CHECK(message_cost >= 0),
  ADD COLUMN reward_cost numeric(12,2) NOT NULL DEFAULT 0 CHECK(reward_cost >= 0),
  ADD COLUMN attribution_window_days integer NOT NULL DEFAULT 7 CHECK(attribution_window_days BETWEEN 1 AND 90),
  ADD COLUMN holdout_percent integer NOT NULL DEFAULT 0 CHECK(holdout_percent BETWEEN 0 AND 50),
  ADD COLUMN holdout_seed varchar(64) NOT NULL DEFAULT encode(gen_random_bytes(16),'hex');

ALTER TABLE campaign_recipients
  ADD COLUMN experiment_group varchar(16) NOT NULL DEFAULT 'treatment' CHECK(experiment_group IN ('treatment','holdout')),
  ADD COLUMN delivered_at timestamptz,
  ADD COLUMN opened_at timestamptz,
  ADD COLUMN clicked_at timestamptz;

CREATE INDEX campaign_recipients_experiment_idx ON campaign_recipients(company_id,campaign_id,experiment_group,status);
CREATE INDEX campaign_recipients_attribution_idx ON campaign_recipients(company_id,customer_id,sent_at DESC) WHERE experiment_group='treatment';

INSERT INTO role_permissions(role,permission) VALUES
 ('owner','campaigns.analytics'),('admin','campaigns.analytics'),('manager','campaigns.analytics')
ON CONFLICT DO NOTHING;
