#!/bin/sh
set -eu

[ "${APP_ENV:-}" = "production" ] || { echo "Refusing bootstrap outside APP_ENV=production" >&2; exit 2; }
: "${DATABASE_URL:?DATABASE_URL is required}"
: "${BOOTSTRAP_ADMIN_EMAIL:?BOOTSTRAP_ADMIN_EMAIL is required}"
: "${BOOTSTRAP_ADMIN_PASSWORD:?BOOTSTRAP_ADMIN_PASSWORD is required}"
[ ${#BOOTSTRAP_ADMIN_PASSWORD} -ge 16 ] || { echo "Bootstrap password must contain at least 16 characters" >&2; exit 2; }
case "$BOOTSTRAP_ADMIN_PASSWORD" in Admin2026\!|Tappix2026\!|DocMed2026\!|*REQUIRED*|*changeme*) echo "Demo/placeholder password refused" >&2; exit 2;; esac
case "$DATABASE_URL" in *sslmode=require*|*sslmode=verify-*) ;; *) echo "Production DATABASE_URL must require TLS" >&2; exit 2;; esac

docker run --rm -i -e DATABASE_URL postgres:17-alpine psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
  -v admin_email="$BOOTSTRAP_ADMIN_EMAIL" -v admin_password="$BOOTSTRAP_ADMIN_PASSWORD" <<'SQL'
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM users WHERE role='super_admin' AND deleted_at IS NULL) THEN
    RAISE EXCEPTION 'an active superadmin already exists; bootstrap is single-use';
  END IF;
END $$;
INSERT INTO users(first_name,last_name,email,password_hash,role,status)
VALUES ('Production','Administrator',lower(:'admin_email'),crypt(:'admin_password',gen_salt('bf')),'super_admin','active');
SQL
echo "Production superadmin created for $BOOTSTRAP_ADMIN_EMAIL. Password was not logged. Unset BOOTSTRAP_ADMIN_PASSWORD now."
