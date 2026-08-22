#!/bin/sh
set -eu
[ "${TAPPIX_TEST_ENV:-}" = "1" ] || { echo "TAPPIX_TEST_ENV=1 is required" >&2; exit 2; }
[ "${APP_ENV:-development}" != "production" ] || { echo "Refusing analytics fixtures in production" >&2; exit 2; }
company=91000000-0000-0000-0000-000000000001
owner=93000000-0000-0000-0000-000000000001
customer=94000000-0000-0000-0000-000000000001
branch_a=92000000-0000-0000-0000-000000000001
branch_b=92000000-0000-0000-0000-000000000002
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-tappix}" -d "${POSTGRES_DB:-tappix}" <<SQL >/dev/null
INSERT INTO companies(id,name,slug,status) VALUES('$company','Stage 3 Analytics','stage3-analytics','active') ON CONFLICT(id) DO NOTHING;
INSERT INTO branches(id,company_id,name,address) VALUES('$branch_a','$company','Branch A','Test A'),('$branch_b','$company','Branch B','Test B') ON CONFLICT(id) DO NOTHING;
INSERT INTO users(id,company_id,first_name,email,password_hash,role,status) VALUES('$owner','$company','Stage3','stage3.analytics@example.test',crypt('Stage3Analytics2026!',gen_salt('bf')),'company_owner','active') ON CONFLICT(id) DO UPDATE SET password_hash=excluded.password_hash,status='active';
INSERT INTO company_memberships(company_id,user_id,role,status) VALUES('$company','$owner','owner','active') ON CONFLICT(company_id,user_id) DO UPDATE SET status='active';
UPDATE subscriptions SET status='cancelled' WHERE company_id='$company' AND status IN('trial','active','past_due');
INSERT INTO subscriptions(company_id,plan_code,status,amount,billing_period,current_period_ends_at) VALUES('$company','pro','active',24990,'monthly',now()+interval '30 days');
INSERT INTO customers(id,company_id,first_name,phone,total_visits) VALUES('$customer','$company','Known dataset','+77000000999',220) ON CONFLICT(id) DO UPDATE SET deleted_at=NULL,total_visits=220;
DELETE FROM visits WHERE company_id='$company';
INSERT INTO visits(company_id,branch_id,customer_id,points_added,created_at) SELECT '$company',CASE WHEN n<=70 THEN '$branch_a'::uuid ELSE '$branch_b'::uuid END,'$customer',0,now()-interval '5 days'+n*interval '1 minute' FROM generate_series(1,120) n;
INSERT INTO visits(company_id,branch_id,customer_id,points_added,created_at) SELECT '$company',CASE WHEN n<=60 THEN '$branch_a'::uuid ELSE '$branch_b'::uuid END,'$customer',0,now()-interval '35 days'+n*interval '1 minute' FROM generate_series(1,100) n;
SQL
api_url="${API_URL:-http://localhost:8080/api/v1}"
token=$(curl -fsS -X POST "$api_url/auth/login" -H 'Content-Type: application/json' -d '{"email":"stage3.analytics@example.test","password":"Stage3Analytics2026!"}' | jq -r '.data.accessToken')
all=$(curl -fsS -H "Authorization: Bearer $token" "$api_url/dashboard?period=30d")
a=$(curl -fsS -H "Authorization: Bearer $token" "$api_url/dashboard?period=30d&branch=$branch_a")
b=$(curl -fsS -H "Authorization: Bearer $token" "$api_url/dashboard?period=30d&branch=$branch_b")
printf '%s' "$all" | jq -e '.data.metrics.visits==120 and .data.previous.visits==100' >/dev/null
printf '%s' "$a" | jq -e '.data.metrics.visits==70 and .data.previous.visits==60' >/dev/null
printf '%s' "$b" | jq -e '.data.metrics.visits==50 and .data.previous.visits==40' >/dev/null
echo "Deterministic analytics: all 120/100, Branch A 70/60, Branch B 50/40 — PASS"
