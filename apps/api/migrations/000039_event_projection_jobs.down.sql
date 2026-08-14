DELETE FROM integration_jobs WHERE connection_id IS NULL;
ALTER TABLE integration_jobs ALTER COLUMN connection_id SET NOT NULL;
