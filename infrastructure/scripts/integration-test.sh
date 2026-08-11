#!/bin/sh
set -eu
API_URL="${API_URL:-http://localhost:8080/api/v1}"
fixture_dir=$(mktemp -d)
trap 'rm -rf "$fixture_dir"' EXIT INT TERM

login=$(curl -fsS -X POST "$API_URL/auth/login" -H 'Content-Type: application/json' -d '{"email":"armat@tappix.kz","password":"Tappix2026!"}')
token=$(printf '%s' "$login" | jq -r '.data.accessToken')
test -n "$token"

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
curl -fsS -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{}' "$API_URL/analytics/refresh" | jq -e '.data.refreshed == true' >/dev/null
assert_get_json bonus-liability /analytics/bonus-liability '.data.issued >= 0 and .data.liability >= 0 and .data.expectedRedemptionCost >= 0'
assert_get_json registration-retention '/analytics/retention?cohortType=registration&periods=4' '.data.grain == "month" and .data.periods == 4 and (.data.cohorts | type == "array")'
assert_get_json purchase-retention '/analytics/retention?cohortType=first_purchase&periods=4' '.data.grain == "week" and (.data.cohorts | type == "array")'
assert_get_json integration-connections /integration-connections '.data | type == "array"'
assert_get_json webhook-deliveries /webhook-deliveries '.data | type == "array"'
assert_get_json campaigns /campaigns '.data | type == "array"'
assert_get_json campaign-automations /campaign-automations '.data | length == 3 and ([.[].triggerType] | sort) == (["birthday_bonus","bonus_expiry_3d","winback_30d"] | sort)'
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

customer_login=$(curl -fsS -X POST "$API_URL/customer/login" -H 'Content-Type: application/json' -d '{"company":"dentline","phone":"+7 700 333 33 33","pin":"1234"}')
customer_token=$(printf '%s' "$customer_login" | jq -r '.data.accessToken')
customer_profile=$(curl -fsS "$API_URL/customer/me" -H "Authorization: Bearer $customer_token")
curl -fsS "$API_URL/customer/rewards" -H "Authorization: Bearer $customer_token" | jq -e '.data | type == "array"' >/dev/null
profile_payload=$(printf '%s' "$customer_profile" | jq -c '{firstName:.data.firstName,lastName:.data.lastName,birthday:(.data.birthday // "" | .[0:10])}')
curl -fsS -X PATCH "$API_URL/customer/me" -H "Authorization: Bearer $customer_token" -H 'Content-Type: application/json' -d "$profile_payload" | jq -e '.data.updated == true' >/dev/null

docmed=$(curl -fsS -X POST "$API_URL/auth/login" -H 'Content-Type: application/json' -d '{"email":"owner@docmed.kz","password":"DocMed2026!"}')
docmed_token=$(printf '%s' "$docmed" | jq -r '.data.accessToken')
dentline_customer=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/customers" | jq -r '.data.items[0].id')
curl -fsS "$API_URL/customers/$dentline_customer/rewards" -H "Authorization: Bearer $token" | jq -e '.data | type == "array"' >/dev/null
cross_status=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $docmed_token" "$API_URL/customers/$dentline_customer")
test "$cross_status" = "404"

branch_id=$(curl -fsS -H "Authorization: Bearer $token" "$API_URL/branches" | jq -r '.data[0].id')
curl -fsS -H "Authorization: Bearer $token" "$API_URL/branches/$branch_id" | jq -e '.data.stats.visits30Days >= 0 and (.data.employees | type == "array") and (.data.devices | type == "array")' >/dev/null
cross_branch=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $docmed_token" "$API_URL/branches/$branch_id")
test "$cross_branch" = "404"

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
