package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type websiteInput struct {
	Headline    string   `json:"headline"`
	Description string   `json:"description"`
	Services    []string `json:"services"`
	Phone       string   `json:"phone"`
	Address     string   `json:"address"`
	Published   bool     `json:"published"`
}
type bookingInput struct {
	BranchID     string `json:"branchId"`
	CustomerName string `json:"customerName"`
	Phone        string `json:"phone"`
	Service      string `json:"service"`
	StartsAt     string `json:"startsAt"`
	Comment      string `json:"comment"`
}
type bookingStatusInput struct {
	Status string `json:"status"`
}
type apiKeyInput struct {
	Name      string   `json:"name"`
	ExpiresAt string   `json:"expiresAt"`
	Scopes    []string `json:"scopes"`
	Sandbox   bool     `json:"sandbox"`
}

func (a *api) getWebsite(w http.ResponseWriter, r *http.Request) {
	// Services must serialise as [] and never null: the panel calls services.join().
	in := websiteInput{Services: []string{}}
	var contacts map[string]any
	err := a.db.QueryRow(r.Context(), `SELECT headline,description,services,contacts,published FROM website_settings WHERE company_id=$1`, companyID(r)).Scan(&in.Headline, &in.Description, &in.Services, &contacts, &in.Published)
	if errors.Is(err, pgx.ErrNoRows) {
		write(w, 200, envelope{Success: true, Data: websiteInput{Services: []string{}}})
		return
	}
	if err != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить настройки сайта")
		return
	}
	if in.Services == nil {
		in.Services = []string{}
	}
	in.Phone, _ = contacts["phone"].(string)
	in.Address, _ = contacts["address"].(string)
	write(w, 200, envelope{Success: true, Data: in})
}
func (a *api) updateWebsite(w http.ResponseWriter, r *http.Request) {
	var in websiteInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Headline) == "" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите заголовок сайта")
		return
	}
	contacts := map[string]string{"phone": in.Phone, "address": in.Address}
	_, err := a.db.Exec(r.Context(), `INSERT INTO website_settings(company_id,headline,description,services,contacts,published) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(company_id) DO UPDATE SET headline=excluded.headline,description=excluded.description,services=excluded.services,contacts=excluded.contacts,published=excluded.published,updated_at=now()`, companyID(r), in.Headline, in.Description, in.Services, contacts, in.Published)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сохранить сайт")
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}
func (a *api) publicWebsite(w http.ResponseWriter, r *http.Request) {
	var companyID, name, headline, description string
	var services []string
	var contacts map[string]any
	err := a.db.QueryRow(r.Context(), `SELECT c.id,c.name,w.headline,w.description,w.services,w.contacts FROM companies c JOIN website_settings w ON w.company_id=c.id WHERE c.slug=$1 AND c.status='active' AND w.published`, r.PathValue("slug")).Scan(&companyID, &name, &headline, &description, &services, &contacts)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "SITE_NOT_FOUND", "Сайт не опубликован")
		return
	}
	if err != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить сайт")
		return
	}
	if services == nil {
		services = []string{}
	}
	rows, err := a.db.Query(r.Context(), `SELECT id,name,address FROM branches WHERE company_id=$1 AND is_active AND deleted_at IS NULL ORDER BY name`, companyID)
	if err != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить филиалы")
		return
	}
	branches := []map[string]string{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, n, address string
			if err := rows.Scan(&id, &n, &address); err != nil {
				rows.Close()
				fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить филиалы")
				return
			}
			branches = append(branches, map[string]string{"id": id, "name": n, "address": address})
		}
		rows.Close()
		if rows.Err() != nil {
			fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить филиалы")
			return
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"company": name, "headline": headline, "description": description, "services": services, "contacts": contacts, "branches": branches}})
}
func (a *api) publicCreateBooking(w http.ResponseWriter, r *http.Request) {
	var in bookingInput
	if !decode(w, r, &in) {
		return
	}
	if in.BranchID == "" || in.CustomerName == "" || in.Service == "" || in.StartsAt == "" {
		fail(w, 422, "VALIDATION_ERROR", "Заполните данные записи")
		return
	}
	if !regexp.MustCompile(`^[+]?\d{7,15}$`).MatchString(strings.TrimSpace(in.Phone)) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите корректный телефон")
		return
	}
	starts, err := time.Parse(time.RFC3339, in.StartsAt)
	if err != nil || starts.Before(time.Now()) {
		fail(w, 422, "VALIDATION_ERROR", "Выберите будущую дату и время")
		return
	}
	var published, moduleEnabled, subscriptionActive bool
	var services []string
	if err = a.db.QueryRow(r.Context(), `SELECT w.published,w.services,EXISTS(SELECT 1 FROM company_modules cm WHERE cm.company_id=c.id AND cm.module_code='booking' AND cm.enabled),EXISTS(SELECT 1 FROM subscriptions s WHERE s.company_id=c.id AND s.status IN ('trial','active','past_due') AND (s.current_period_ends_at IS NULL OR s.current_period_ends_at>now())) FROM companies c LEFT JOIN website_settings w ON w.company_id=c.id JOIN branches b ON b.company_id=c.id WHERE c.slug=$1 AND b.id=$2 AND c.status='active' AND b.is_active`, r.PathValue("slug"), in.BranchID).Scan(&published, &services, &moduleEnabled, &subscriptionActive); err != nil {
		fail(w, 404, "BRANCH_NOT_FOUND", "Компания или филиал не найдены")
		return
	}
	if !published {
		fail(w, 404, "SITE_NOT_FOUND", "Сайт не опубликован")
		return
	}
	if !moduleEnabled || !subscriptionActive {
		fail(w, 403, "ENTITLEMENT_REQUIRED", "Бронирование недоступно по тарифу")
		return
	}
	validService := false
	for _, service := range services {
		if service == in.Service {
			validService = true
			break
		}
	}
	if !validService {
		fail(w, 422, "VALIDATION_ERROR", "Услуга недоступна на сайте")
		return
	}
	var id string
	err = a.db.QueryRow(r.Context(), `INSERT INTO bookings(company_id,branch_id,customer_name,phone,service,starts_at,comment)
		SELECT c.id,b.id,$3,$4,$5,$6,$7 FROM companies c JOIN branches b ON b.company_id=c.id
		JOIN website_settings w ON w.company_id=c.id AND w.published AND w.services @> jsonb_build_array($5::text)
		WHERE c.slug=$1 AND b.id=$2 AND c.status='active' AND b.is_active
		AND EXISTS (SELECT 1 FROM company_modules cm WHERE cm.company_id=c.id AND cm.module_code='booking' AND cm.enabled)
		AND EXISTS (SELECT 1 FROM subscriptions s WHERE s.company_id=c.id AND s.status IN ('trial','active','past_due') AND (s.current_period_ends_at IS NULL OR s.current_period_ends_at>now()))
		RETURNING id`, r.PathValue("slug"), in.BranchID, in.CustomerName, in.Phone, in.Service, starts, in.Comment).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			fail(w, 409, "SLOT_UNAVAILABLE", "Это время уже занято")
			return
		}
		fail(w, 404, "BRANCH_NOT_FOUND", "Компания или филиал не найдены")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": id, "status": "new"}})
}
func (a *api) listBookings(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT bk.id,bk.customer_name,bk.phone,bk.service,bk.starts_at,bk.status,coalesce(bk.comment,''),b.name FROM bookings bk JOIN branches b ON b.id=bk.branch_id WHERE bk.company_id=$1 ORDER BY bk.starts_at DESC LIMIT 200`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить записи")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, phone, service, status, comment, branch string
		var starts time.Time
		if err := rows.Scan(&id, &name, &phone, &service, &starts, &status, &comment, &branch); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить записи")
			return
		}
		items = append(items, map[string]any{"id": id, "customerName": name, "phone": phone, "service": service, "startsAt": starts, "status": status, "comment": comment, "branch": branch})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить записи")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) updateBooking(w http.ResponseWriter, r *http.Request) {
	var in bookingStatusInput
	if !decode(w, r, &in) {
		return
	}
	if in.Status != "new" && in.Status != "confirmed" && in.Status != "completed" && in.Status != "cancelled" {
		fail(w, 422, "VALIDATION_ERROR", "Некорректный статус записи")
		return
	}
	if in.Status != "cancelled" {
		var current string
		if err := a.db.QueryRow(r.Context(), `SELECT status FROM bookings WHERE company_id=$1 AND id=$2`, companyID(r), r.PathValue("id")).Scan(&current); err != nil {
			fail(w, 404, "BOOKING_NOT_FOUND", "Запись не найдена")
			return
		}
		if current == "cancelled" {
			fail(w, 409, "INVALID_BOOKING_TRANSITION", "Отменённую запись нельзя активировать")
			return
		}
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE bookings SET status=$3,updated_at=now() WHERE company_id=$1 AND id=$2`, companyID(r), r.PathValue("id"), in.Status)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "BOOKING_NOT_FOUND", "Запись не найдена")
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}
func (a *api) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id,name,prefix,scopes,sandbox,last_used_at,expires_at,revoked_at,created_at FROM api_keys WHERE company_id=$1 ORDER BY created_at DESC`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить API-ключи")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, prefix string
		var scopes []string
		var sandbox bool
		var last, expires, revoked *time.Time
		var created time.Time
		if err := rows.Scan(&id, &name, &prefix, &scopes, &sandbox, &last, &expires, &revoked, &created); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить API-ключи")
			return
		}
		items = append(items, map[string]any{"id": id, "name": name, "prefix": prefix, "scopes": scopes, "sandbox": sandbox, "lastUsedAt": last, "expiresAt": expires, "revokedAt": revoked, "createdAt": created, "active": revoked == nil && (expires == nil || expires.After(time.Now()))})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить API-ключи")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var in apiKeyInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите название ключа")
		return
	}
	allowedScopes := map[string]bool{"transactions.read": true, "transactions.write": true, "transactions.refund": true, "jobs.retry": true}
	if len(in.Scopes) == 0 {
		in.Scopes = []string{"transactions.read", "transactions.write"}
	}
	for _, scope := range in.Scopes {
		if !allowedScopes[scope] {
			fail(w, 422, "INVALID_SCOPE", "API-ключ содержит неизвестный scope")
			return
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		fail(w, 500, "KEY_ERROR", "Не удалось создать ключ")
		return
	}
	secret := "tpx_" + base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(secret))
	var expires any
	if in.ExpiresAt != "" {
		expires = in.ExpiresAt
	}
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO api_keys(company_id,name,prefix,secret_hash,expires_at,scopes,sandbox) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, companyID(r), strings.TrimSpace(in.Name), secret[:12], hex.EncodeToString(sum[:]), expires, in.Scopes, in.Sandbox).Scan(&id)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сохранить ключ")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "key": secret, "scopes": in.Scopes, "sandbox": in.Sandbox, "warning": "Скопируйте ключ сейчас — повторно он не показывается"}})
}
func (a *api) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tag, err := a.db.Exec(r.Context(), `UPDATE api_keys SET revoked_at=now() WHERE company_id=$1 AND id=$2 AND revoked_at IS NULL`, companyID(r), r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "API_KEY_NOT_FOUND", "Ключ не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"revoked": true}})
}
