CREATE TABLE report_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  schedule_id uuid NOT NULL REFERENCES report_schedules(id) ON DELETE CASCADE,
  idempotency_key varchar(180) NOT NULL,
  status varchar(24) NOT NULL CHECK(status IN ('queued','processing','sent','skipped','failed')),
  format varchar(16) NOT NULL CHECK(format IN ('summary','csv','xlsx','pdf')),
  filename varchar(240),
  mime_type varchar(120),
  artifact bytea,
  attempts integer NOT NULL DEFAULT 0,
  error text,
  started_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(company_id,idempotency_key)
);

CREATE INDEX report_runs_schedule_idx ON report_runs(company_id,schedule_id,created_at DESC);
CREATE INDEX report_runs_queue_idx ON report_runs(status,created_at) WHERE status IN ('queued','processing');

ALTER TABLE report_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON report_runs
  USING (company_id = nullif(current_setting('app.company_id', true), '')::uuid)
  WITH CHECK (company_id = nullif(current_setting('app.company_id', true), '')::uuid);
