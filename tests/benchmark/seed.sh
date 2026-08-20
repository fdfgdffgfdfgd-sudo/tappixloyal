#!/bin/sh
set -eu
: "${TAPPIX_TEST_ENV:?set TAPPIX_TEST_ENV=1}"
[ "$TAPPIX_TEST_ENV" = 1 ] || exit 1
: "${BENCHMARK_DATABASE_URL:?set BENCHMARK_DATABASE_URL explicitly}"
case "$BENCHMARK_DATABASE_URL" in *tappix_bench*) :;; *) echo 'benchmark database required' >&2; exit 1;; esac
COUNT="${1:?usage: seed.sh COUNT}"
case "$COUNT" in *[!0-9]*|'') exit 1;; esac
psql_cmd() { if command -v psql >/dev/null 2>&1; then psql "$BENCHMARK_DATABASE_URL" "$@"; else docker compose exec -T postgres psql -U tappix -d tappix_bench "$@"; fi; }
psql_cmd -v ON_ERROR_STOP=1 -v count="$COUNT" <<'SQL'
CREATE EXTENSION IF NOT EXISTS pgcrypto;
INSERT INTO companies(id,name,slug,status) VALUES ('00000000-0000-0000-0000-000000000001','Benchmark','benchmark','active');
INSERT INTO users(id,company_id,email,first_name,last_name,password_hash,role,status) VALUES ('00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000001','bench@example.test','Bench','Owner','$2a$04$rQP8wUTilHd5e7ibrarcq.RiIcYGr14fPyXZMcc.85s0Yag9LRAta','company_owner','active');
INSERT INTO branches(id,company_id,name,address,is_active) VALUES ('00000000-0000-0000-0000-000000000002','00000000-0000-0000-0000-000000000001','Benchmark Branch','Local',true);
INSERT INTO company_memberships(company_id,user_id,role,status) VALUES ('00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000001','owner','active');
INSERT INTO subscriptions(company_id,plan_code,status,amount,current_period_ends_at) VALUES ('00000000-0000-0000-0000-000000000001','pro','active',0,now()+interval '30 days');
INSERT INTO company_modules(company_id,module_code,enabled) SELECT '00000000-0000-0000-0000-000000000001',code,true FROM modules WHERE code IN ('core','crm','loyalty','booking','website');
INSERT INTO reward_definitions(company_id,name,reward_type,value,created_by,created_at)
SELECT '00000000-0000-0000-0000-000000000001','Benchmark reward '||g,'gift',0,'00000000-0000-0000-0000-000000000001',TIMESTAMPTZ '2025-01-01 00:00:00Z'+g*INTERVAL '1 second' FROM generate_series(1, :count) g;
INSERT INTO reward_rules(company_id,definition_id,event_type,threshold,created_at)
SELECT company_id,id,'visit_milestone',6,created_at FROM reward_definitions WHERE company_id='00000000-0000-0000-0000-000000000001';
SQL
psql_cmd -v ON_ERROR_STOP=1 -Atc "SELECT 'reward_definitions='||count(*) FROM reward_definitions WHERE company_id='00000000-0000-0000-0000-000000000001'; SELECT 'reward_rules='||count(*) FROM reward_rules WHERE company_id='00000000-0000-0000-0000-000000000001';"
