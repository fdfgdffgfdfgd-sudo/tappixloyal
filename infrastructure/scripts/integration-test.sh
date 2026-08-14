#!/bin/sh
set -eu
API_URL="${API_URL:-http://localhost:8080/api/v1}"
fixture_dir=$(mktemp -d)
trap 'rm -rf "$fixture_dir"' EXIT INT TERM

login=$(curl -fsS -X POST "$API_URL/auth/login" -H 'Content-Type: application/json' -d '{"email":"armat@tappix.kz","password":"Tappix2026!"}')
token=$(printf '%s' "$login" | jq -r '.data.accessToken')
test -n "$token"
curl -fsS -H "Authorization: Bearer $token" "$API_URL/dashboard" | jq -e '.data.repeatCustomers >= 0 and .data.rewardsIssued >= 0 and (.data.latestCustomers | type == "array")' >/dev/null
curl -fsS "${API_URL%/api/v1}/metrics" | grep -q 'tappix_http_requests_total'

assert_get_json() {
	label=$1
	path=$2
	filter=$3
	body="$fixture_dir/response.json"
	status=$(curl -sS -o "$body" -w '%{http_code}' -H "Authorization: Bearer $token" "$API_URL$path")
	if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
		printf 'Integration check failed: %s (GET %s) returned HTTP %s\n' "$label" "$path" "$status" >&2
		cat "$body" >&2
		return 1
	fi
	if ! jq -e "$filter" "$body" >/dev/null; then
		printf 'Integration check failed: %s (GET %s) returned an unexpected payload\n' "$label" "$path" >&2
		cat "$body" >&2
		return 1
	fi
}

curl -fsS -H "Authorization: Bearer $token" "$API_URL/auth/sessions" | jq -e '.data | type == "array" and length >= 1' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/workspaces" | jq -e '.data | type == "array" and any(.current == true and .name == "Dentline")' >/dev/null
forgot_unknown=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API_URL/auth/forgot-password" -H 'Content-Type: application/json' -d '{"email":"missing-account@example.com"}')
test "$forgot_unknown" = "200"
expired_reset=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API_URL/auth/reset-password" -H 'Content-Type: application/json' -d '{"token":"invalid-token","newPassword":"ValidPass2026!"}')
test "$expired_reset" = "410"

unauthorized=$(curl -sS -o /dev/null -w '%{http_code}' "$API_URL/customers")
test "$unauthorized" = "401"
canonical_unauthorized=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API_URL/integrations/transactions/quote" -H 'Content-Type: application/json' -d '{}')
test "$canonical_unauthorized" = "401"

assert_get_json dashboard /dashboard '.success == true and (.data.latestCustomers | type == "array") and (.data.latestVisits | type == "array") and (.data.onboarding | type == "object") and .data.bonusRedeemed >= 0 and .data.nfcConversion >= 0'
assert_get_json business-analytics /analytics/business '.success == true and (.data.repeatPurchase.windows | length == 3) and (.data.averageCheck.overall >= 0) and (.data.ltv.type == "historical") and (.data.rfm.segments | type == "array") and (.data.branches | type == "array") and (.data.funnel | length == 7)'
assert_get_json business-outcomes /analytics/outcomes?days=30 '.data.days == 30 and .data.retention.returnedCustomers >= 0 and .data.automations.attributedRevenue >= 0 and .data.referrals.newCustomers >= 0 and (.data.rewards.bestName | type == "string") and .data.previous.returnedCustomers >= 0 and (.data.branches | type == "array")'
curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{}' "$API_URL/analytics/refresh" | jq -e '.data.refreshed == true' >/dev/null
assert_get_json bonus-liability /analytics/bonus-liability '.data.issued >= 0 and .data.liability >= 0 and .data.expectedRedemptionCost >= 0'
assert_get_json registration-retention '/analytics/retention?cohortType=registration&periods=4' '.data.grain == "month" and .data.periods == 4 and (.data.cohorts | type == "array")'
assert_get_json purchase-retention '/analytics/retention?cohortType=first_purchase&periods=4' '.data.grain == "week" and (.data.cohorts | type == "array")'
assert_get_json report-schedules /reports/schedules '.data | type == "array"'
report_schedule=$(curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"name":"Integration owner report","reportType":"owner_summary","channel":"email","recipients":["owner@example.com"],"frequency":"weekly","timezone":"Asia/Almaty","sendHour":9,"sendWeekday":1,"format":"xlsx","active":true}' "$API_URL/reports/schedules")
report_schedule_id=$(printf '%s' "$report_schedule" | jq -r '.data.id')
report_run=$(curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{}' "$API_URL/reports/schedules/$report_schedule_id/run")
report_run_id=$(printf '%s' "$report_run" | jq -r '.data.id')
i=0
report_status=queued
while [ "$i" -lt 30 ] && { [ "$report_status" = queued ] || [ "$report_status" = processing ]; }; do
	sleep 1
	report_status=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/reports/runs" | jq -r --arg id "$report_run_id" '.data[] | select(.id==$id) | .status')
	i=$((i+1))
done
test "$report_status" = sent
curl -fsS -H "Authorization: Bearer $token" "$API_URL/reports/runs/$report_run_id/download" -o "$fixture_dir/report.xlsx"
unzip -t "$fixture_dir/report.xlsx" >/dev/null
curl -fsS -X DELETE -H "Authorization: Bearer $token" "$API_URL/reports/schedules/$report_schedule_id" | jq -e '.data.deleted == true' >/dev/null

# Temporary delivery errors are queued for an automatic retry with a visible next attempt.
retry_schedule=$(curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"name":"Integration retry report","reportType":"owner_summary","channel":"webhook","recipients":["00000000-0000-0000-0000-000000000099"],"frequency":"daily","timezone":"Asia/Almaty","sendHour":9,"format":"summary","active":true}' "$API_URL/reports/schedules")
retry_schedule_id=$(printf '%s' "$retry_schedule" | jq -r '.data.id')
retry_run=$(curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{}' "$API_URL/reports/schedules/$retry_schedule_id/run")
retry_run_id=$(printf '%s' "$retry_run" | jq -r '.data.id')
i=0
retry_attempts=0
while [ "$i" -lt 30 ] && [ "$retry_attempts" -lt 1 ]; do
	sleep 1
	retry_attempts=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/reports/runs" | jq -r --arg id "$retry_run_id" '.data[] | select(.id==$id) | .attempts')
	i=$((i+1))
done
curl -fsS -H "Authorization: Bearer $token" "$API_URL/reports/runs" | jq -e --arg id "$retry_run_id" '.data[] | select(.id==$id) | .status=="queued" and .attempts==1 and (.nextAttemptAt|length)>0 and (.error|length)>0' >/dev/null
curl -fsS -X DELETE -H "Authorization: Bearer $token" "$API_URL/reports/schedules/$retry_schedule_id" | jq -e '.data.deleted == true' >/dev/null
assert_get_json integration-connections /integration-connections '.data | type == "array"'
assert_get_json webhook-deliveries /webhook-deliveries '.data | type == "array"'
assert_get_json campaigns /campaigns '.data | type == "array"'
assert_get_json campaign-automations /campaign-automations '.data | length == 6 and ([.[].triggerType] | sort) == (["birthday_bonus","bonus_expiry_3d","winback_30d","near_reward","reward_unlocked","nfc_registration"] | sort)'
invalid_holdout=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API_URL/campaigns" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"name":"Invalid holdout","subject":"Test","body":"Test","segment":"all","holdoutPercent":3}')
test "$invalid_holdout" = "422"
curl -fsS -H "Authorization: Bearer $token" "$API_URL/loyalty/inactive?days=30" | jq -e '.data.total >= 0 and (.data.items | type == "array")' >/dev/null
curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{}' "$API_URL/loyalty/process-birthdays" | jq -e '.data.processed >= 0' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers" | jq -e '.data.total >= 1' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers?level=basic" | jq -e '.data.items | all(.level == "basic")' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers?sort=points&order=desc&minPoints=100&limit=2" | jq -e '.data.limit == 2 and (.data.items | all(.totalPoints >= 100))' >/dev/null
bad_email=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API_URL/customers" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"firstName":"Bad email","phone":"+7 700 000 99 99","email":"invalid"}')
test "$bad_email" = "422"
header_injection=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API_URL/notifications/send" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"recipient":"test@example.com","subject":"Hello\r\nBcc: victim@example.com","body":"test"}')
test "$header_injection" = "422"
export_headers=$(curl -sS -D - -o "$fixture_dir/customers.csv" -H "Authorization: Bearer $token" "$API_URL/customers/export?level=basic")
printf '%s' "$export_headers" | tr -d '\r' | grep -qi '^Content-Type: text/csv'
grep -q 'Имя,Фамилия,Телефон' "$fixture_dir/customers.csv"

guest_cookies="$fixture_dir/guest-cookies.txt"
customer_login=$(curl -fsS -c "$guest_cookies" -X POST "$API_URL/customer/login" -H 'Content-Type: application/json' -d '{"company":"dentline","phone":"+7 700 333 33 33","pin":"1234"}')
printf '%s' "$customer_login" | jq -e '.data.authenticated == true and (.data.accessToken | not)' >/dev/null
grep -q 'tappix_guest_access' "$guest_cookies"
grep -q 'tappix_guest_refresh' "$guest_cookies"
guest_csrf=$(awk '$6 == "tappix_guest_csrf" { print $7 }' "$guest_cookies")
test -n "$guest_csrf"
customer_profile=$(curl -fsS -b "$guest_cookies" "$API_URL/customer/me")
curl -fsS -b "$guest_cookies" "$API_URL/customer/wallet" | jq -e '.data.loyalty.mode and (.data.loyalty.progress >= 0) and (.data.loyalty.remaining >= 0) and (.data.loyalty.eligible | type == "boolean")' >/dev/null
curl -fsS -b "$guest_cookies" "$API_URL/customer/rewards" | jq -e '.data | type == "array"' >/dev/null
profile_payload=$(printf '%s' "$customer_profile" | jq -c '{firstName:.data.firstName,lastName:.data.lastName,birthday:(.data.birthday // "" | .[0:10])}')
csrf_rejected=$(curl -sS -o /dev/null -w '%{http_code}' -b "$guest_cookies" -X PATCH "$API_URL/customer/me" -H 'Content-Type: application/json' -d "$profile_payload")
test "$csrf_rejected" = "403"
curl -fsS -b "$guest_cookies" -X PATCH "$API_URL/customer/me" -H "X-CSRF-Token: $guest_csrf" -H 'Content-Type: application/json' -d "$profile_payload" | jq -e '.data.updated == true' >/dev/null
curl -fsS -b "$guest_cookies" -c "$guest_cookies" -H "X-CSRF-Token: $guest_csrf" -X POST "$API_URL/auth/refresh?aud=guest" | jq -e '.success == true' >/dev/null
guest_csrf=$(awk '$6 == "tappix_guest_csrf" { print $7 }' "$guest_cookies")
curl -fsS -b "$guest_cookies" -c "$guest_cookies" -H "X-CSRF-Token: $guest_csrf" -X POST "$API_URL/auth/logout?aud=guest" | jq -e '.data.loggedOut == true' >/dev/null
guest_logged_out=$(curl -sS -o /dev/null -w '%{http_code}' -b "$guest_cookies" "$API_URL/customer/me")
test "$guest_logged_out" = "401"

platform_cookies="$fixture_dir/platform-cookies.txt"
platform_login=$(curl -fsS -c "$platform_cookies" -X POST "$API_URL/auth/login" -H 'Content-Type: application/json' -d '{"email":"admin@tappix.kz","password":"Admin2026!"}')
printf '%s' "$platform_login" | jq -e '(.data.mfaSetupRequired == true and .data.user.role == "super_admin") or (.data.mfaRequired == true and (.data.mfaChallenge | length) > 0)' >/dev/null
platform_blocked=$(curl -sS -o /dev/null -w '%{http_code}' -b "$platform_cookies" "$API_URL/admin/dashboard")
if printf '%s' "$platform_login" | jq -e '.data.mfaSetupRequired == true' >/dev/null; then
  test "$platform_blocked" = "403"
  platform_csrf=$(awk '$6 == "tappix_platform_csrf" { print $7 }' "$platform_cookies")
  curl -fsS -b "$platform_cookies" -H "X-CSRF-Token: $platform_csrf" -X POST "$API_URL/auth/mfa/setup?aud=platform" -H 'Content-Type: application/json' -d '{}' | jq -e '.data.secret | length >= 16' >/dev/null
else
  test "$platform_blocked" = "401"
fi

docmed=$(curl -fsS -X POST "$API_URL/auth/login" -H 'Content-Type: application/json' -d '{"email":"owner@docmed.kz","password":"DocMed2026!"}')
docmed_token=$(printf '%s' "$docmed" | jq -r '.data.accessToken')
dentline_customer=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers" | jq -r '.data.items[0].id')
dentline_code=$(docker compose exec -T postgres psql -U tappix -d tappix -Atc "SELECT customer_code FROM customers WHERE id='$dentline_customer'")
test "${#dentline_code}" = "6"
curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"code\":\"$dentline_code\"}" "$API_URL/staff/customers/lookup" | jq -e --arg id "$dentline_customer" '.data.id == $id and (.data.phoneMasked | contains("•••"))' >/dev/null
curl -fsS "$API_URL/customers/$dentline_customer/rewards" -H "Authorization: Bearer $token" | jq -e '.data | type == "array"' >/dev/null
curl -fsS "$API_URL/customers/$dentline_customer/timeline" -H "Authorization: Bearer $token" | jq -e '.data | type == "array"' >/dev/null
cross_status=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $docmed_token" "$API_URL/customers/$dentline_customer")
test "$cross_status" = "404"
cross_risk=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $docmed_token" "$API_URL/customers/$dentline_customer/risk")
test "$cross_risk" = "404"
cross_code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST -H "Authorization: Bearer $docmed_token" -H 'Content-Type: application/json' -d "{\"code\":\"$dentline_code\"}" "$API_URL/staff/customers/lookup")
test "$cross_code" = "404"
docker compose exec -T postgres psql -U tappix -d tappix -Atc "SELECT count(*) FROM audit_logs WHERE action='customer.code_lookup' AND company_id=(SELECT id FROM companies WHERE slug='dentline')" | grep -Eq '^[1-9][0-9]*$'

branch_id=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/branches" | jq -r '.data[0].id')
curl -fsS -H "Authorization: Bearer $token" "$API_URL/branches/$branch_id" | jq -e '.data.stats.visits30Days >= 0 and (.data.employees | type == "array") and (.data.devices | type == "array")' >/dev/null
cross_branch=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $docmed_token" "$API_URL/branches/$branch_id")
test "$cross_branch" = "404"

# Product DoD: publish program -> register guest -> wallet progress -> staff visit -> reward -> redeem -> history.
original_portal=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/settings/guest-portal" | jq -c '.data + {loyaltyMode:(.data.loyaltyMode // "points"),stampsTarget:(.data.stampsTarget // 6),stampReward:(.data.stampReward // "Подарок") }')
curl -fsS -X PATCH -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"loyaltyMode":"stamps","stampsTarget":2,"stampReward":"Integration подарок","discountStart":3,"discountStep":2,"discountMax":15,"visitsPerStep":3}' "$API_URL/settings/guest-portal" | jq -e '.data.loyaltyMode == "stamps" and .data.stampsTarget == 2' >/dev/null
device_token=$(docker compose exec -T postgres psql -U tappix -d tappix -Atc "SELECT token FROM devices WHERE company_id=(SELECT id FROM companies WHERE slug='dentline') AND is_active LIMIT 1")
product_phone="+7 701 8$(date +%H%M%S)"
product_cookies="$fixture_dir/product-cookies.txt"
registered=$(curl -fsS -c "$product_cookies" -X POST "$API_URL/customer/register" -H 'Content-Type: application/json' -d "{\"token\":\"$device_token\",\"firstName\":\"Product\",\"lastName\":\"DoD\",\"phone\":\"$product_phone\",\"pin\":\"4827\",\"consent\":true}")
product_customer=$(printf '%s' "$registered" | jq -r '.data.customerId')
docker compose exec -T postgres psql -U tappix -d tappix -c "UPDATE customers SET total_visits=1 WHERE id='$product_customer'" >/dev/null
curl -fsS -b "$product_cookies" "$API_URL/customer/wallet" | jq -e '.data.loyalty.mode == "stamps" and .data.loyalty.progress == 1 and .data.loyalty.remaining == 1 and .data.loyalty.rewardTitle == "Integration подарок"' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers/$product_customer/timeline" | jq -e '.data | any(.type == "customer.registered")' >/dev/null
visit=$(curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"customerId\":\"$product_customer\",\"branchId\":\"$branch_id\",\"comment\":\"Product DoD\"}" "$API_URL/visits")
printf '%s' "$visit" | jq -e '.data.totalVisits == 2 and .data.reward == "1 наград"' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers/$product_customer" | jq -e '.data.favoriteBranch != "" and .data.lastBranch == .data.favoriteBranch' >/dev/null
duplicate_visit=$(curl -sS -o "$fixture_dir/duplicate-visit.json" -w '%{http_code}' -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"customerId\":\"$product_customer\",\"branchId\":\"$branch_id\",\"comment\":\"Duplicate\"}" "$API_URL/visits")
test "$duplicate_visit" = "409"
jq -e '.error.code == "DUPLICATE_VISIT"' "$fixture_dir/duplicate-visit.json" >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers/$product_customer/risk" | jq -e '.data | any(.operation == "visit.create" and .severity == "blocked" and .status == "open")' >/dev/null
docker compose exec -T postgres psql -U tappix -d tappix -Atc "SELECT count(*) FROM audit_logs WHERE company_id=(SELECT id FROM companies WHERE slug='dentline') AND action='security.operation_blocked' AND entity_id='$product_customer'" | grep -Eq '^[1-9][0-9]*$'
reward_id=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers/$product_customer/rewards" | jq -r '.data[] | select(.name=="Integration подарок" and .status=="available") | .id' | head -1)
test -n "$reward_id"
curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"reason\":\"Product DoD redemption\",\"branchId\":\"$branch_id\",\"idempotencyKey\":\"product-dod-redemption\"}" "$API_URL/rewards/$reward_id/redeem" | jq -e '.data.status == "redeemed"' >/dev/null
curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"reason\":\"Product DoD redemption\",\"branchId\":\"$branch_id\",\"idempotencyKey\":\"product-dod-redemption\"}" "$API_URL/rewards/$reward_id/redeem" | jq -e '.data.status == "redeemed" and .data.idempotentReplay == true' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/rewards/$reward_id/transactions" | jq -e '.data | any(.operation == "issued") and any(.operation == "redeemed")' >/dev/null
docker compose exec -T postgres psql -U tappix -d tappix -Atc "SELECT count(*) FROM reward_transactions WHERE reward_id='$reward_id' AND operation='redeemed'" | grep -qx '1'
curl -fsS -b "$product_cookies" "$API_URL/customer/rewards" | jq -e '.data | any(.name == "Integration подарок" and .status == "redeemed")' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers/$product_customer/timeline" | jq -e '.data as $events | ([$events[].type] | contains(["visit.completed","bonus.earned","reward.unlocked","reward.redeemed"])) and any($events[]; .type == "reward.redeemed" and .branch != "")' >/dev/null
expired_reward=$(docker compose exec -T postgres psql -U tappix -d tappix -Atc "INSERT INTO customer_rewards(company_id,customer_id,name,status,expires_at) VALUES((SELECT id FROM companies WHERE slug='dentline'),'$product_customer','Просроченная тестовая награда','available',now()-interval '1 minute') RETURNING id" | head -1)
test -n "$expired_reward"
curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{}' "$API_URL/rewards/expire" | jq -e '.data.expired >= 1' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers/$product_customer/timeline" | jq -e '.data | any(.type == "reward.expired" and .properties.name == "Просроченная тестовая награда")' >/dev/null
curl -fsS -X PATCH -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$original_portal" "$API_URL/settings/guest-portal" >/dev/null
curl -fsS -X DELETE -H "Authorization: Bearer $token" "$API_URL/customers/$product_customer" | jq -e '.data.archived == true' >/dev/null

device=$(curl -fsS -X POST "$API_URL/devices" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"branchId\":\"$branch_id\",\"kind\":\"qr\",\"name\":\"Integration device\",\"destination\":\"join\"}")
device_id=$(printf '%s' "$device" | jq -r '.data.id')
curl -fsS -X PATCH "$API_URL/devices/$device_id" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"branchId\":\"$branch_id\",\"kind\":\"nfc\",\"name\":\"Updated integration device\",\"destination\":\"reviews\",\"active\":false}" | jq -e '.data.updated == true' >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$API_URL/devices" | jq -e --arg id "$device_id" '.data | any(.id == $id and .kind == "nfc" and .destination == "reviews" and .active == false)' >/dev/null
curl -fsS -X DELETE "$API_URL/devices/$device_id" -H "Authorization: Bearer $token" | jq -e '.data.deleted == true' >/dev/null

customer=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers/$dentline_customer")
balance=$(printf '%s' "$customer" | jq -r '.data.totalPoints')
overdraft=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $token" -X POST "$API_URL/customers/$dentline_customer/bonus" -H 'Content-Type: application/json' -d "{\"operation\":\"debit\",\"amount\":$((balance+1)),\"description\":\"integration overdraft\"}")
test "$overdraft" = "409"

bad_webhook=$(curl -sS -o /dev/null -w '%{http_code}' -X PATCH "$API_URL/settings/integrations" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"webhookUrl":"http://unsafe.local"}')
test "$bad_webhook" = "422"
curl -fsS -X PATCH "$API_URL/settings/integrations" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"telegramEnabled":true,"smsEnabled":false,"webhookUrl":"https://example.com/tappix","crmName":"amoCRM"}' | jq -e '.data.telegramEnabled == true and .data.crmName == "amoCRM"' >/dev/null
curl -fsS "$API_URL/settings/integrations" -H "Authorization: Bearer $token" | jq -e '.data.webhookUrl == "https://example.com/tappix"' >/dev/null

printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=' | base64 -d > "$fixture_dir/pixel.png"
printf '%s' 'not an allowed file' > "$fixture_dir/file.txt"
bad_file=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API_URL/upload" -H "Authorization: Bearer $token" -F 'kind=document' -F "file=@$fixture_dir/file.txt")
test "$bad_file" = "422"
uploaded=$(curl -fsS -X POST "$API_URL/upload" -H "Authorization: Bearer $token" -F 'kind=asset' -F "file=@$fixture_dir/pixel.png")
file_id=$(printf '%s' "$uploaded" | jq -r '.data.id')
test -n "$file_id"
curl -fsS "$API_URL/files" -H "Authorization: Bearer $token" | jq -e --arg id "$file_id" '.data | any(.id == $id)' >/dev/null
public_file=$(curl -sS -o /dev/null -w '%{http_code}' "$API_URL/public/files/$file_id")
test "$public_file" = "200"
curl -fsS -X DELETE "$API_URL/files/$file_id" -H "Authorization: Bearer $token" | jq -e '.data.deleted == true' >/dev/null
deleted_file=$(curl -sS -o /dev/null -w '%{http_code}' "$API_URL/public/files/$file_id")
test "$deleted_file" = "404"

# Paid modules are controlled only by Platform Owner; company owners cannot self-enable them.
module_escalation=$(curl -sS -o /dev/null -w '%{http_code}' -X PATCH "$API_URL/modules/whatsapp" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"enabled":true}')
test "$module_escalation" = "403"

printf '%s\n' 'Tappix integration test: PASS'
