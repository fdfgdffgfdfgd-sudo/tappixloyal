CREATE TABLE IF NOT EXISTS company_memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('owner','admin','manager','staff')),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('invited','active','suspended')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id,user_id)
);

INSERT INTO company_memberships(company_id,user_id,role,status)
SELECT company_id,id,CASE WHEN role='company_owner' THEN 'owner' ELSE 'staff' END,'active'
FROM users WHERE company_id IS NOT NULL AND role IN ('company_owner','employee')
ON CONFLICT(company_id,user_id) DO NOTHING;

CREATE INDEX IF NOT EXISTS company_memberships_user_idx ON company_memberships(user_id,status);
ALTER TABLE company_memberships ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON company_memberships;
CREATE POLICY tenant_isolation ON company_memberships USING (company_id=current_setting('app.company_id',true)::uuid);
