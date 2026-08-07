ALTER TABLE users
  ADD COLUMN IF NOT EXISTS mfa_secret text,
  ADD COLUMN IF NOT EXISTS mfa_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS mfa_enabled_at timestamptz;

CREATE TABLE IF NOT EXISTS role_permissions (
  role text NOT NULL CHECK (role IN ('owner','admin','manager','staff')),
  permission text NOT NULL,
  PRIMARY KEY (role, permission)
);

INSERT INTO role_permissions(role,permission) VALUES
 ('owner','workspace.read'),('owner','customers.read'),('owner','customers.write'),('owner','customers.delete'),
 ('owner','visits.read'),('owner','visits.write'),('owner','bonus.write'),('owner','rewards.write'),
 ('owner','analytics.read'),('owner','settings.read'),('owner','settings.write'),('owner','staff.manage'),
 ('owner','billing.read'),('owner','devices.read'),('owner','devices.manage'),('owner','files.read'),('owner','files.manage'),
 ('admin','workspace.read'),('admin','customers.read'),('admin','customers.write'),('admin','customers.delete'),
 ('admin','visits.read'),('admin','visits.write'),('admin','bonus.write'),('admin','rewards.write'),
 ('admin','analytics.read'),('admin','settings.read'),('admin','settings.write'),('admin','staff.manage'),
 ('admin','devices.read'),('admin','devices.manage'),('admin','files.read'),('admin','files.manage'),
 ('manager','workspace.read'),('manager','customers.read'),('manager','customers.write'),
 ('manager','visits.read'),('manager','visits.write'),('manager','bonus.write'),('manager','rewards.write'),
 ('manager','analytics.read'),('manager','devices.read'),('manager','files.read'),
 ('staff','workspace.read'),('staff','customers.read'),('staff','customers.write'),
 ('staff','visits.read'),('staff','visits.write'),('staff','bonus.write'),('staff','rewards.write'),('staff','devices.read')
ON CONFLICT DO NOTHING;

CREATE INDEX IF NOT EXISTS support_sessions_actor_idx ON support_sessions(actor_id,created_at DESC);
CREATE INDEX IF NOT EXISTS support_sessions_active_idx ON support_sessions(company_id,expires_at) WHERE revoked_at IS NULL;
