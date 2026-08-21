-- Revert only the permission rows removed by the forward migration.
INSERT INTO role_permissions(role, permission)
VALUES ('staff','analytics.read')
ON CONFLICT DO NOTHING;
