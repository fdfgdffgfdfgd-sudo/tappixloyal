DELETE FROM role_permissions WHERE permission='rewards.read' AND role IN ('owner','admin');
