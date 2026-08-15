package httpapi

import (
	"encoding/json"
	"net/http"
	"regexp"
)

var sixDigitCustomerCode = regexp.MustCompile(`^[0-9]{6}$`)

func (a *api) customerByCode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input) != nil {
		fail(w, 422, "INVALID_CUSTOMER_CODE", "Введите 6 цифр")
		return
	}
	code := input.Code
	if !sixDigitCustomerCode.MatchString(code) {
		fail(w, 422, "INVALID_CUSTOMER_CODE", "Введите 6 цифр")
		return
	}
	claims := identity(r)
	var id, first, last, phone string
	var points, visits int
	err := a.db.QueryRow(r.Context(), `SELECT id,first_name,last_name,phone,total_points,total_visits FROM customers WHERE company_id=$1 AND customer_code=$2 AND deleted_at IS NULL`, claims.CompanyID, code).Scan(&id, &first, &last, &phone, &points, &visits)
	host := clientIP(r)
	outcome := "found"
	var entityID any = id
	if err != nil {
		outcome = "not_found"
		entityID = nil
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,ip,user_agent,after_data) VALUES($1,$2,'customer.code_lookup','customer',$3,$4,$5,jsonb_build_object('outcome',$6::text))`, claims.CompanyID, claims.Subject, entityID, host, r.UserAgent(), outcome)
	logDomainEvent(r, "staff.customer.lookup", id, "outcome", outcome)
	if err != nil {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Клиент с таким кодом не найден")
		return
	}
	masked := phone
	if len(phone) > 4 {
		masked = "+7 ••• •• " + phone[len(phone)-4:len(phone)-2] + " " + phone[len(phone)-2:]
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": id, "firstName": first, "lastName": last, "phoneMasked": masked, "totalPoints": points, "totalVisits": visits}})
}
