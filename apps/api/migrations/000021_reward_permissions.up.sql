INSERT INTO role_permissions(role,permission) VALUES
 ('owner','rewards.read'),('admin','rewards.read')
ON CONFLICT DO NOTHING;
