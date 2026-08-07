#!/bin/sh
set -eu

BASE_URL=${BASE_URL:-http://localhost:8088}
API="$BASE_URL/api/v1"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

expect_code(){ actual=$1; expected=$2; label=$3; [ "$actual" = "$expected" ] || { echo "FAIL: $label (got $actual, want $expected)"; exit 1; }; }

code=$(curl -sS -o /dev/null -w '%{http_code}' "$API/customers")
expect_code "$code" 401 "protected endpoint without authentication"

headers=$(curl -sSI "$BASE_URL/login")
printf '%s' "$headers" | grep -qi 'content-security-policy:' || { echo 'FAIL: CSP header missing'; exit 1; }
printf '%s' "$headers" | grep -qi 'x-content-type-options: nosniff' || { echo 'FAIL: nosniff missing'; exit 1; }
printf '%s' "$headers" | grep -qi 'x-frame-options: sameorigin' || { echo 'FAIL: frame protection missing'; exit 1; }

login_headers=$(curl -sS -D "$work/login.headers" -c "$work/owner.cookies" -o "$work/login.json" -X POST "$API/auth/login" -H 'Content-Type: application/json' -d '{"email":"armat@tappix.kz","password":"Tappix2026!"}')
owner_token=$(jq -r '.data.accessToken' "$work/login.json")
grep -qi 'httponly' "$work/login.headers" || { echo 'FAIL: HttpOnly cookie missing'; exit 1; }
grep -qi 'samesite=strict' "$work/login.headers" || { echo 'FAIL: SameSite Strict missing'; exit 1; }

code=$(curl -sS -H "Authorization: Bearer $owner_token" -o /dev/null -w '%{http_code}' "$API/admin/dashboard")
expect_code "$code" 403 "business owner reaching founder API"

code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"' OR 1=1 --\",\"password\":\"x\"}")
[ "$code" = 401 ] || [ "$code" = 422 ] || { echo "FAIL: injection-shaped login accepted ($code)"; exit 1; }

docmed_id=$(docker compose exec -T postgres psql -U tappix -d tappix -Atc "SELECT cu.id FROM customers cu JOIN companies c ON c.id=cu.company_id WHERE c.slug='docmed' LIMIT 1")
code=$(curl -sS -H "Authorization: Bearer $owner_token" -o /dev/null -w '%{http_code}' "$API/customers/$docmed_id")
expect_code "$code" 404 "cross-tenant customer access"

# Controlled local burst: validates throttling without generating sustained load.
limited=0
i=0
while [ "$i" -lt 140 ]; do
  code=$(curl -sS -o /dev/null -w '%{http_code}' "$API/security-rate-probe")
  if [ "$code" = 429 ] || [ "$code" = 503 ]; then limited=1; break; fi
  i=$((i+1))
done
[ "$limited" = 1 ] || { echo 'FAIL: rate limit did not engage during controlled burst'; exit 1; }

echo 'Tappix security test: PASS'
