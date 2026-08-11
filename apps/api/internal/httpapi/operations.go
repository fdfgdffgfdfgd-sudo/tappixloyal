package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type employeeInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	BranchID  string `json:"branchId"`
	Status    string `json:"status"`
}
type deviceInput struct {
	BranchID    string `json:"branchId"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Destination string `json:"destination"`
	Active      *bool  `json:"active"`
}

func (a *api) listEmployees(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT u.id,u.first_name,u.last_name,u.email,u.role,u.status,u.last_login_at,u.branch_id,b.name FROM users u LEFT JOIN branches b ON b.id=u.branch_id WHERE u.company_id=$1 AND u.deleted_at IS NULL ORDER BY u.created_at`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить сотрудников")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, first, last, email, role, status string
		var login *time.Time
		var branchID, branch *string
		if rows.Scan(&id, &first, &last, &email, &role, &status, &login, &branchID, &branch) == nil {
			items = append(items, map[string]any{"id": id, "firstName": first, "lastName": last, "email": email, "role": role, "status": status, "lastLoginAt": login, "branchId": branchID, "branch": branch})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) createEmployee(w http.ResponseWriter, r *http.Request) {
	var in employeeInput
	if !decode(w, r, &in) {
		return
	}
	if ok, limit := a.checkLimit(r.Context(), companyID(r), "staff"); !ok {
		fail(w, 409, "LIMIT_REACHED", fmt.Sprintf("Достигнут лимит сотрудников: %d", limit.Used))
		return
	}
	if strings.TrimSpace(in.FirstName) == "" || !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите имя, email и пароль от 8 символов")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "PASSWORD_ERROR", "Не удалось обработать пароль")
		return
	}
	var branch any
	if in.BranchID != "" {
		branch = in.BranchID
	}
	var id string
	err = a.db.QueryRow(r.Context(), `INSERT INTO users(company_id,branch_id,first_name,last_name,email,password_hash,role,status) VALUES($1,$2,$3,$4,$5,$6,'employee','active') RETURNING id`, companyID(r), branch, strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName), strings.TrimSpace(in.Email), string(hash)).Scan(&id)
	if err != nil {
		fail(w, 409, "EMPLOYEE_EXISTS", "Сотрудник с таким email уже существует")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": id}})
}
func (a *api) updateEmployee(w http.ResponseWriter, r *http.Request) {
	var in employeeInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.FirstName) == "" || !strings.Contains(in.Email, "@") {
		fail(w, 422, "VALIDATION_ERROR", "Укажите имя и корректный email")
		return
	}
	if in.Status != "active" && in.Status != "blocked" {
		fail(w, 422, "VALIDATION_ERROR", "Статус должен быть active или blocked")
		return
	}
	var branch any
	if in.BranchID != "" {
		var exists bool
		if err := a.db.QueryRow(r.Context(), `SELECT exists(SELECT 1 FROM branches WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL)`, companyID(r), in.BranchID).Scan(&exists); err != nil || !exists {
			fail(w, 404, "BRANCH_NOT_FOUND", "Филиал не найден")
			return
		}
		branch = in.BranchID
	}
	var hash any
	if in.Password != "" {
		if len(in.Password) < 8 {
			fail(w, 422, "VALIDATION_ERROR", "Пароль должен содержать минимум 8 символов")
			return
		}
		encoded, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			fail(w, 500, "PASSWORD_ERROR", "Не удалось обработать пароль")
			return
		}
		hash = string(encoded)
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE users SET first_name=$3,last_name=$4,email=$5,branch_id=$6,status=$7,password_hash=coalesce($8,password_hash),updated_at=now() WHERE company_id=$1 AND id=$2 AND role='employee' AND deleted_at IS NULL`, companyID(r), r.PathValue("id"), strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName), strings.TrimSpace(in.Email), branch, in.Status, hash)
	if err != nil {
		fail(w, 409, "EMPLOYEE_EXISTS", "Сотрудник с таким email уже существует")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "EMPLOYEE_NOT_FOUND", "Сотрудник не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"updated": true}})
}
func (a *api) deleteEmployee(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	if r.PathValue("id") == claims.Subject {
		fail(w, 409, "CANNOT_DELETE_SELF", "Нельзя удалить собственный аккаунт")
		return
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE users SET deleted_at=now(),status='blocked' WHERE company_id=$1 AND id=$2 AND role='employee' AND deleted_at IS NULL`, companyID(r), r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "EMPLOYEE_NOT_FOUND", "Сотрудник не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"archived": true}})
}

func (a *api) getSubscription(w http.ResponseWriter, r *http.Request) {
	var id, plan, status, currency, period string
	var amount float64
	var start time.Time
	var end *time.Time
	err := a.db.QueryRow(r.Context(), `SELECT id,plan_code,status,amount,currency,billing_period,starts_at,current_period_ends_at FROM subscriptions WHERE company_id=$1 AND status IN('trial','active','past_due') ORDER BY created_at DESC LIMIT 1`, companyID(r)).Scan(&id, &plan, &status, &amount, &currency, &period, &start, &end)
	if errors.Is(err, pgx.ErrNoRows) {
		write(w, 200, envelope{Success: true, Data: map[string]any{"plan": "Starter", "status": "trial", "amount": 0, "currency": "KZT", "modules": []string{"core"}}})
		return
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить подписку")
		return
	}
	planCode := normalizePlanCode(plan)
	modules := []string{}
	rows, _ := a.db.Query(r.Context(), `SELECT module_code FROM company_modules WHERE company_id=$1 AND enabled ORDER BY module_code`, companyID(r))
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var code string
			if rows.Scan(&code) == nil {
				modules = append(modules, code)
			}
		}
	}
	entitlements := map[string]any{}
	if planCode != "" {
		entitlementRows, _ := a.db.Query(r.Context(), `SELECT code,enabled,limit_value FROM plan_entitlements WHERE plan_code=$1 ORDER BY code`, planCode)
		if entitlementRows != nil {
			defer entitlementRows.Close()
			for entitlementRows.Next() {
				var code string
				var enabled bool
				var limit *int
				if entitlementRows.Scan(&code, &enabled, &limit) == nil {
					entitlements[code] = map[string]any{"enabled": enabled, "limit": limit}
				}
			}
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": id, "plan": plan, "tier": planCode, "status": status, "amount": amount, "currency": currency, "billingPeriod": period, "startsAt": start, "currentPeriodEndsAt": end, "modules": modules, "entitlements": entitlements}})
}

func (a *api) listDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT d.id,d.branch_id,b.name,d.kind,d.name,d.token,d.destination,d.is_active,d.scans_count,d.last_scanned_at FROM devices d JOIN branches b ON b.id=d.branch_id WHERE d.company_id=$1 ORDER BY d.created_at DESC`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить устройства")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, branchID, branch, kind, name, token, destination string
		var active bool
		var scans int64
		var last *time.Time
		if rows.Scan(&id, &branchID, &branch, &kind, &name, &token, &destination, &active, &scans, &last) == nil {
			publicURL := strings.TrimRight(envOr("APP_URL", "http://localhost:8088"), "/") + "/join/" + token
			items = append(items, map[string]any{"id": id, "branchId": branchID, "branch": branch, "kind": kind, "name": name, "token": token, "url": publicURL, "destination": destination, "active": active, "scans": scans, "lastScannedAt": last})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) createDevice(w http.ResponseWriter, r *http.Request) {
	var in deviceInput
	if !decode(w, r, &in) {
		return
	}
	if ok, limit := a.checkLimit(r.Context(), companyID(r), "smart_links"); !ok {
		fail(w, 409, "LIMIT_REACHED", fmt.Sprintf("Достигнут лимит NFC/QR-точек: %d", limit.Used))
		return
	}
	if in.Kind != "nfc" && in.Kind != "qr" {
		fail(w, 422, "VALIDATION_ERROR", "Тип должен быть nfc или qr")
		return
	}
	if in.BranchID == "" || strings.TrimSpace(in.Name) == "" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите филиал и название")
		return
	}
	if !validDestination(in.Destination) {
		fail(w, 422, "VALIDATION_ERROR", "Некорректное назначение устройства")
		return
	}
	destination := in.Destination
	if destination == "" {
		destination = "join"
	}
	var id, token string
	err := a.db.QueryRow(r.Context(), `INSERT INTO devices(company_id,branch_id,kind,name,destination) SELECT $1,b.id,$3,$4,$5 FROM branches b WHERE b.id=$2 AND b.company_id=$1 AND b.deleted_at IS NULL RETURNING id,token`, companyID(r), in.BranchID, in.Kind, strings.TrimSpace(in.Name), destination).Scan(&id, &token)
	if err != nil {
		fail(w, 404, "BRANCH_NOT_FOUND", "Филиал не найден")
		return
	}
	publicURL := strings.TrimRight(envOr("APP_URL", "http://localhost:8088"), "/") + "/join/" + token
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": id, "token": token, "url": publicURL}})
}
func (a *api) updateDevice(w http.ResponseWriter, r *http.Request) {
	var in deviceInput
	if !decode(w, r, &in) {
		return
	}
	if in.Kind != "nfc" && in.Kind != "qr" || in.BranchID == "" || strings.TrimSpace(in.Name) == "" || in.Destination == "" || !validDestination(in.Destination) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите тип, название, филиал и назначение")
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE devices d SET branch_id=b.id,kind=$4,name=$5,destination=$6,is_active=$7,updated_at=now() FROM branches b WHERE d.company_id=$1 AND d.id=$2 AND b.company_id=$1 AND b.id=$3 AND b.deleted_at IS NULL`, companyID(r), r.PathValue("id"), in.BranchID, in.Kind, strings.TrimSpace(in.Name), in.Destination, active)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "DEVICE_NOT_FOUND", "Устройство не найдено")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"updated": true}})
}
func validDestination(value string) bool {
	return value == "" || value == "join" || value == "reviews" || value == "website" || value == "booking"
}
func (a *api) deleteDevice(w http.ResponseWriter, r *http.Request) {
	tag, err := a.db.Exec(r.Context(), `DELETE FROM devices WHERE company_id=$1 AND id=$2`, companyID(r), r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "DEVICE_NOT_FOUND", "Устройство не найдено")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"deleted": true}})
}
