package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var sixDigitCustomerCode = regexp.MustCompile(`^[0-9]{6}$`)
var customerUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func (a *api) customerByCode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code       string `json:"code"`
		CustomerID string `json:"customerId"`
		Phone      string `json:"phone"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input) != nil {
		fail(w, 422, "INVALID_CUSTOMER_LOOKUP", "Введите код клиента, QR или телефон")
		return
	}
	code := strings.ReplaceAll(strings.TrimSpace(input.Code), " ", "")
	customerID := strings.TrimSpace(input.CustomerID)
	phoneDigits := nonDigits.ReplaceAllString(input.Phone, "")
	lookupCount := 0
	if code != "" {
		lookupCount++
	}
	if customerID != "" {
		lookupCount++
	}
	if phoneDigits != "" {
		lookupCount++
	}
	if lookupCount != 1 || (code != "" && !sixDigitCustomerCode.MatchString(code)) || (customerID != "" && !customerUUID.MatchString(customerID)) || (phoneDigits != "" && len(phoneDigits) < 10) {
		fail(w, 422, "INVALID_CUSTOMER_LOOKUP", "Введите один корректный код клиента, QR или телефон")
		return
	}
	claims := identity(r)
	var id, first, last, phone string
	var points, visits int
	err := a.db.QueryRow(r.Context(), `SELECT id,first_name,last_name,phone,total_points,total_visits FROM customers
		WHERE company_id=$1 AND deleted_at IS NULL AND
		(($2<>'' AND customer_code=$2) OR ($3<>'' AND id=nullif($3,'')::uuid) OR ($4<>'' AND regexp_replace(phone,'[^0-9]','','g')=$4))`, claims.CompanyID, code, customerID, phoneDigits).Scan(&id, &first, &last, &phone, &points, &visits)
	host := clientIP(r)
	outcome := "found"
	var entityID any = id
	if err != nil {
		outcome = "not_found"
		entityID = nil
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,ip,user_agent,after_data) VALUES($1,$2,'customer.code_lookup','customer',$3,$4,$5,jsonb_build_object('outcome',$6::text))`, claims.CompanyID, claims.Subject, entityID, host, r.UserAgent(), outcome)
	logDomainEvent(r, "staff.customer.lookup", id, "outcome", outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Клиент с таким кодом не найден")
		return
	}
	if err != nil {
		fail(w, 500, "CUSTOMER_LOOKUP_FAILED", "Не удалось найти клиента")
		return
	}
	progress := []map[string]any{}
	progressRows, err := a.db.Query(r.Context(), `SELECT rd.name,p.current_value,p.target_value,p.status
		FROM customer_reward_progress p JOIN reward_rules rr ON rr.id=p.rule_id AND rr.company_id=p.company_id
		JOIN reward_definitions rd ON rd.id=rr.definition_id AND rd.company_id=p.company_id
		WHERE p.company_id=$1 AND p.customer_id=$2 AND p.status IN('in_progress','available','completed') ORDER BY p.updated_at DESC LIMIT 3`, claims.CompanyID, id)
	if err != nil {
		fail(w, 500, "CUSTOMER_PROGRESS_FAILED", "Не удалось загрузить прогресс клиента")
		return
	}
	for progressRows.Next() {
		var name, status string
		var current, target int
		if err = progressRows.Scan(&name, &current, &target, &status); err != nil {
			progressRows.Close()
			fail(w, 500, "CUSTOMER_PROGRESS_FAILED", "Не удалось загрузить прогресс клиента")
			return
		}
		progress = append(progress, map[string]any{"name": name, "currentValue": current, "targetValue": target, "status": status})
	}
	progressRows.Close()
	if err = progressRows.Err(); err != nil {
		fail(w, 500, "CUSTOMER_PROGRESS_FAILED", "Не удалось загрузить прогресс клиента")
		return
	}
	rewards := []map[string]any{}
	rewardRows, err := a.db.Query(r.Context(), `SELECT cr.id,cr.name,
		CASE WHEN cr.status IN('available','reserved') AND cr.expires_at<=now() THEN 'expired'
		WHEN cr.status='reserved' AND cr.reserved_until<=now() THEN 'available' ELSE cr.status END,
		cr.expires_at,cr.redeemed_at FROM customer_rewards cr
		WHERE cr.company_id=$1 AND cr.customer_id=$2 AND cr.status IN('available','reserved','redeemed')
		ORDER BY CASE cr.status WHEN 'available' THEN 0 WHEN 'reserved' THEN 1 ELSE 2 END,cr.issued_at DESC LIMIT 10`, claims.CompanyID, id)
	if err != nil {
		fail(w, 500, "CUSTOMER_REWARDS_FAILED", "Не удалось загрузить награды клиента")
		return
	}
	for rewardRows.Next() {
		var rewardID, name, status string
		var expiresAt, redeemedAt *time.Time
		if err = rewardRows.Scan(&rewardID, &name, &status, &expiresAt, &redeemedAt); err != nil {
			rewardRows.Close()
			fail(w, 500, "CUSTOMER_REWARDS_FAILED", "Не удалось загрузить награды клиента")
			return
		}
		rewards = append(rewards, map[string]any{"id": rewardID, "name": name, "status": status, "expiresAt": expiresAt, "redeemedAt": redeemedAt})
	}
	rewardRows.Close()
	if err = rewardRows.Err(); err != nil {
		fail(w, 500, "CUSTOMER_REWARDS_FAILED", "Не удалось загрузить награды клиента")
		return
	}
	masked := phone
	if len(phone) > 4 {
		masked = "+7 ••• •• " + phone[len(phone)-4:len(phone)-2] + " " + phone[len(phone)-2:]
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": id, "firstName": first, "lastName": last, "phoneMasked": masked, "totalPoints": points, "totalVisits": visits, "rewardProgress": progress, "rewards": rewards}})
}
