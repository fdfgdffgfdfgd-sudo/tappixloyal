DROP TABLE IF EXISTS role_permissions;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_enabled_at, DROP COLUMN IF EXISTS mfa_enabled, DROP COLUMN IF EXISTS mfa_secret;
