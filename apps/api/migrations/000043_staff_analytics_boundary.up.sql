-- Staff Mode is an operational surface.  Reporting and analytics stay owner/manager-only.
-- Keep this as a data migration so existing installations are corrected on upgrade.
DELETE FROM role_permissions
WHERE role = 'staff'
  AND permission IN ('analytics.read', 'bonus_liability.read');

