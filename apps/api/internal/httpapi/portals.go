package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
	"golang.org/x/crypto/bcrypt"
)

type customerRegisterInput struct {
	Token        string `json:"token"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	Phone        string `json:"phone"`
	Birthday     string `json:"birthday"`
	Gender       string `json:"gender"`
	Email        string `json:"email"`
	City         string `json:"city"`
	Consent      bool   `json:"consent"`
	PIN          string `json:"pin"`
	ReferralCode string `json:"referralCode"`
}
type customerLoginInput struct {
	Company string `json:"company"`
	Phone   string `json:"phone"`
	PIN     string `json:"pin"`
}
type adminCompanyInput struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	OwnerFirstName string `json:"ownerFirstName"`
	OwnerEmail     string `json:"ownerEmail"`
	Password       string `json:"password"`
}

func (a *api) customerRegister(w http.ResponseWriter, r *http.Request) {
	var in customerRegisterInput
	if !decode(w, r, &in) {
		return
	}
	if in.Token == "" || strings.TrimSpace(in.FirstName) == "" || strings.TrimSpace(in.LastName) == "" || len(in.Phone) < 7 || !in.Consent {
		fail(w, 422, "VALIDATION_ERROR", "Заполните имя, фамилию, телефон и подтвердите согласие")
		return
	}
	var company, device, branch string
	err := a.db.QueryRow(r.Context(), `UPDATE devices SET scans_count=scans_count+1,last_scanned_at=now() WHERE token=$1 AND is_active RETURNING company_id,id,coalesce(branch_id::text,'')`, in.Token).Scan(&company, &device, &branch)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "DEVICE_NOT_FOUND", "Ссылка регистрации недействительна")
		return
	}
	pin := strings.TrimSpace(in.PIN)
	if len(pin) < 4 {
		pin = tokenHash(fmt.Sprintf("%s:%s:%d", company, in.Phone, time.Now().UnixNano()))[:12]
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать регистрацию")
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	var points int
	var birthday any
	if in.Birthday != "" {
		birthday = in.Birthday
	}
	err = tx.QueryRow(r.Context(), `SELECT id,total_points FROM customers WHERE company_id=$1 AND phone=$2 AND deleted_at IS NULL FOR UPDATE`, company, in.Phone).Scan(&id, &points)
	created := false
	if errors.Is(err, pgx.ErrNoRows) {
		if ok, limit := a.checkLimit(r.Context(), company, "customers"); !ok {
			fail(w, 409, "LIMIT_REACHED", fmt.Sprintf("Программа временно не принимает новые карты: лимит %d", limit.Used))
			return
		}
		created = true
		welcome := a.pointsForEvent(r.Context(), company, "customer_registered", 0)
		err = tx.QueryRow(r.Context(), `INSERT INTO customers(company_id,first_name,last_name,phone,birthday,gender,email,city,pin_hash,total_points) VALUES($1,$2,$3,$4,$5,nullif($6,''),nullif($7,''),nullif($8,''),$9,$10) RETURNING id`, company, strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName), in.Phone, birthday, strings.TrimSpace(in.Gender), strings.TrimSpace(in.Email), strings.TrimSpace(in.City), string(hash), welcome).Scan(&id)
		points = welcome
		if err == nil && welcome > 0 {
			var ledgerID string
			err = tx.QueryRow(r.Context(), `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description) VALUES($1,$2,'credit',$3,$3,'Приветственный бонус') RETURNING id`, company, id, welcome).Scan(&ledgerID)
			if err == nil {
				err = posintegration.IssueBonusLot(r.Context(), tx, company, id, ledgerID, "", welcome)
			}
		}
	} else if err == nil {
		// Re-opening an existing card must never replace its authentication secret.
		// Registration may refresh profile fields, but PIN changes belong to an
		// authenticated account-management flow.
		_, err = tx.Exec(r.Context(), `UPDATE customers SET first_name=$3,last_name=$4,birthday=coalesce($5,birthday),gender=coalesce(nullif($6,''),gender),email=coalesce(nullif($7,''),email),city=coalesce(nullif($8,''),city),updated_at=now() WHERE company_id=$1 AND id=$2`, company, id, strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName), birthday, strings.TrimSpace(in.Gender), strings.TrimSpace(in.Email), strings.TrimSpace(in.City))
	}
	if err == nil && created && strings.TrimSpace(in.ReferralCode) != "" {
		code := strings.ToUpper(strings.TrimSpace(in.ReferralCode))
		var attributionID string
		err = tx.QueryRow(r.Context(), `SELECT a.id FROM referral_attributions a JOIN referral_programs p ON p.id=a.program_id AND p.company_id=a.company_id AND p.status='active'
			WHERE a.company_id=$1 AND a.referral_code=$2 AND a.referred_customer_id IS NULL AND a.status='clicked' ORDER BY a.clicked_at DESC LIMIT 1 FOR UPDATE`, company, code).Scan(&attributionID)
		if err == nil {
			_, err = tx.Exec(r.Context(), `UPDATE referral_attributions SET referred_customer_id=$3,status='registered',registered_at=now(),updated_at=now() WHERE company_id=$1 AND id=$2 AND referrer_customer_id<>$3`, company, attributionID, id)
		} else if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(r.Context(), `INSERT INTO referral_attributions(company_id,program_id,referrer_customer_id,referred_customer_id,referral_code,status,registered_at,source)
				SELECT c.company_id,p.id,c.id,$3,c.referral_code,'registered',now(),'direct' FROM customers c JOIN referral_programs p ON p.company_id=c.company_id AND p.status='active'
				WHERE c.company_id=$1 AND c.referral_code=$2 AND c.id<>$3 RETURNING id`, company, code, id).Scan(&attributionID)
			if errors.Is(err, pgx.ErrNoRows) {
				err = nil
			}
		}
	}
	if err == nil && created {
		err = appendCustomerEvent(r, tx, company, id, "customer.registered", branch, "customer-registered:"+id, map[string]any{"deviceId": device, "channel": "nfc_qr"})
	}
	if err == nil && created && points > 0 {
		err = appendCustomerEvent(r, tx, company, id, "bonus.earned", branch, "welcome-bonus:"+id, map[string]any{"amount": points, "balanceAfter": points, "reason": "Приветственный бонус"})
	}
	if err != nil {
		slog.Error("customer.registration.failed", "event_type", "customer.registration.failed", "tenant_id", company, "request_id", r.Header.Get("X-Request-ID"), "error", err)
		fail(w, 500, "REGISTRATION_FAILED", "Не удалось зарегистрироваться")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		slog.Error("customer.registration.commit_failed", "event_type", "customer.registration.commit_failed", "tenant_id", company, "request_id", r.Header.Get("X-Request-ID"), "error", err)
		fail(w, 500, "REGISTRATION_FAILED", "Не удалось зарегистрироваться")
		return
	}
	access, refresh, err := a.issueTokens(r, id, company, "customer")
	if err != nil {
		fail(w, 500, "TOKEN_ERROR", "Не удалось создать сессию")
		return
	}
	a.setSessionCookies(w, access, refresh, "customer")
	write(w, 201, envelope{Success: true, Data: map[string]any{"customerId": id, "points": points, "created": created, "deviceId": device}})
}

func (a *api) customerLogin(w http.ResponseWriter, r *http.Request) {
	var in customerLoginInput
	if !decode(w, r, &in) {
		return
	}
	var id, company, hash string
	err := a.db.QueryRow(r.Context(), `SELECT c.id,c.company_id,c.pin_hash FROM customers c JOIN companies co ON co.id=c.company_id WHERE co.slug=$1 AND c.phone=$2 AND c.deleted_at IS NULL`, strings.ToLower(in.Company), in.Phone).Scan(&id, &company, &hash)
	lockKey := "customer-pin-lock:" + strings.ToLower(in.Company) + ":" + in.Phone
	failKey := "customer-pin-fail:" + strings.ToLower(in.Company) + ":" + in.Phone
	if a.redis != nil {
		if locked, _ := a.redis.Exists(r.Context(), lockKey).Result(); locked > 0 {
			fail(w, http.StatusTooManyRequests, "PIN_LOCKED", "Слишком много неверных попыток. Попробуйте позже")
			return
		}
	}
	if err != nil || hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.PIN)) != nil {
		if a.redis != nil {
			count, _ := a.redis.Incr(r.Context(), failKey).Result()
			if count == 1 { _ = a.redis.Expire(r.Context(), failKey, 15*time.Minute).Err() }
			if count >= 5 { _ = a.redis.Set(r.Context(), lockKey, "1", 15*time.Minute).Err() }
		}
		fail(w, 401, "INVALID_CREDENTIALS", "Неверный телефон или PIN")
		return
	}
	if a.redis != nil { _ = a.redis.Del(r.Context(), failKey).Err() }
	access, refresh, err := a.issueTokens(r, id, company, "customer")
	if err != nil {
		fail(w, 500, "TOKEN_ERROR", "Не удалось создать сессию")
		return
	}
	a.setSessionCookies(w, access, refresh, "customer")
	write(w, 200, envelope{Success: true, Data: map[string]bool{"authenticated": true}})
}
func (a *api) customerMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var first, last, phone, level, company, companySlug, logo, companyPhone, address, email, city, gender string
	var birthday *time.Time
	var points, visits int
	var lastVisit *time.Time
	var branding map[string]any
	err := a.db.QueryRow(r.Context(), `SELECT c.first_name,c.last_name,c.phone,c.birthday,c.total_points,c.total_visits,c.level,co.name,co.slug,coalesce(co.logo_url,''),coalesce(co.phone,''),coalesce(co.address,''),coalesce(c.email,''),coalesce(c.city,''),coalesce(c.gender,''),(SELECT max(v.created_at) FROM visits v WHERE v.company_id=c.company_id AND v.customer_id=c.id),coalesce(cs.branding,'{}'::jsonb) FROM customers c JOIN companies co ON co.id=c.company_id LEFT JOIN company_settings cs ON cs.company_id=co.id WHERE c.company_id=$1 AND c.id=$2`, claims.CompanyID, claims.Subject).Scan(&first, &last, &phone, &birthday, &points, &visits, &level, &company, &companySlug, &logo, &companyPhone, &address, &email, &city, &gender, &lastVisit, &branding)
	if err != nil {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Профиль не найден")
		return
	}
	guest, _ := branding["guestPortal"].(map[string]any)
	var reviewURL string
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce(nullif(gis_url,''),nullif(google_url,''),nullif(yandex_url,''),'') FROM review_settings WHERE company_id=$1 AND enabled`, claims.CompanyID).Scan(&reviewURL)
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": claims.Subject, "firstName": first, "lastName": last, "phone": phone, "birthday": birthday, "email": email, "city": city, "gender": gender, "points": points, "visits": visits, "lastVisit": lastVisit, "level": level, "company": company, "companySlug": companySlug, "logoUrl": logo, "companyPhone": companyPhone, "address": address, "reviewUrl": reviewURL, "portal": guest}})
}

func (a *api) publicGuestPortal(w http.ResponseWriter, r *http.Request) {
	var companyID, company, slug, logo, phone, address string
	var branding map[string]any
	err := a.db.QueryRow(r.Context(), `SELECT c.id,c.name,c.slug,coalesce(c.logo_url,''),coalesce(c.phone,''),coalesce(c.address,''),coalesce(cs.branding,'{}'::jsonb) FROM devices d JOIN companies c ON c.id=d.company_id AND c.status='active' LEFT JOIN company_settings cs ON cs.company_id=c.id WHERE d.token=$1 AND d.is_active`, r.PathValue("token")).Scan(&companyID, &company, &slug, &logo, &phone, &address, &branding)
	if err != nil {
		fail(w, 404, "DEVICE_NOT_FOUND", "Ссылка программы лояльности недействительна")
		return
	}
	portal, _ := branding["guestPortal"].(map[string]any)
	write(w, 200, envelope{Success: true, Data: map[string]any{"companyId": companyID, "company": company, "slug": slug, "logoUrl": logo, "phone": phone, "address": address, "portal": portal, "welcomeBonus": a.pointsForEvent(r.Context(), companyID, "customer_registered", 0)}})
}

func (a *api) getGuestPortalSettings(w http.ResponseWriter, r *http.Request) {
	var portal map[string]any
	err := a.db.QueryRow(r.Context(), `SELECT coalesce(branding->'guestPortal','{}'::jsonb) FROM company_settings WHERE company_id=$1`, companyID(r)).Scan(&portal)
	if errors.Is(err, pgx.ErrNoRows) {
		portal = map[string]any{}
	} else if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить Guest Portal")
		return
	}
	write(w, 200, envelope{Success: true, Data: portal})
}

func (a *api) updateGuestPortalSettings(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	if !decode(w, r, &in) {
		return
	}
	mode, _ := in["loyaltyMode"].(string)
	target := intConfig(in, "stampsTarget", 6)
	reward := strings.TrimSpace(stringConfig(in, "stampReward", "Подарок"))
	if mode != "points" && mode != "stamps" && mode != "discount" {
		fail(w, 422, "VALIDATION_ERROR", "Выберите тип программы")
		return
	}
	if mode == "stamps" && (target < 2 || target > 30 || reward == "") {
		fail(w, 422, "VALIDATION_ERROR", "Укажите награду и порог от 2 до 30 посещений")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать публикацию")
		return
	}
	defer tx.Rollback(r.Context())
	var definitionID, ruleID *string
	_ = tx.QueryRow(r.Context(), `SELECT loyalty_reward_definition_id,loyalty_reward_rule_id FROM company_settings WHERE company_id=$1 FOR UPDATE`, companyID(r)).Scan(&definitionID, &ruleID)
	if mode == "stamps" {
		if definitionID == nil {
			var id string
			err = tx.QueryRow(r.Context(), `INSERT INTO reward_definitions(company_id,name,description,reward_type,validity_days,repeatable,confirmation_method,is_active,created_by) VALUES($1,$2,'Награда основной программы посещений','gift',90,true,'staff',true,$3) RETURNING id`, companyID(r), reward, identity(r).Subject).Scan(&id)
			definitionID = &id
		} else {
			_, err = tx.Exec(r.Context(), `UPDATE reward_definitions SET name=$3,is_active=true,deleted_at=NULL,updated_at=now() WHERE company_id=$1 AND id=$2`, companyID(r), *definitionID, reward)
		}
		if err == nil {
			if ruleID == nil {
				var id string
				err = tx.QueryRow(r.Context(), `INSERT INTO reward_rules(company_id,definition_id,event_type,threshold,progress_mode,priority,is_active) VALUES($1,$2,'visit_created',$3,'repeat',10,true) RETURNING id`, companyID(r), *definitionID, target).Scan(&id)
				ruleID = &id
			} else {
				_, err = tx.Exec(r.Context(), `UPDATE reward_rules SET definition_id=$3,threshold=$4,progress_mode='repeat',is_active=true,updated_at=now() WHERE company_id=$1 AND id=$2`, companyID(r), *ruleID, *definitionID, target)
			}
		}
	} else if ruleID != nil {
		_, err = tx.Exec(r.Context(), `UPDATE reward_rules SET is_active=false,updated_at=now() WHERE company_id=$1 AND id=$2`, companyID(r), *ruleID)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO company_settings(company_id,branding,loyalty_reward_definition_id,loyalty_reward_rule_id) VALUES($1,jsonb_build_object('guestPortal',$2::jsonb),$3,$4) ON CONFLICT(company_id) DO UPDATE SET branding=company_settings.branding || jsonb_build_object('guestPortal',$2::jsonb),loyalty_reward_definition_id=$3,loyalty_reward_rule_id=$4,updated_at=now()`, companyID(r), in, definitionID, ruleID)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сохранить Guest Portal")
		return
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data) VALUES($1,$2,'loyalty.program.published','loyalty_program',$1,$3,jsonb_build_object('mode',$4::text,'target',$5::integer,'reward',$6::text))`, companyID(r), identity(r).Subject, r.Header.Get("X-Request-ID"), mode, target, reward)
	logDomainEvent(r, "loyalty.program.published", "", "mode", mode, "target", target)
	write(w, 200, envelope{Success: true, Data: in})
}
func (a *api) updateCustomerProfile(w http.ResponseWriter, r *http.Request) {
	var in customerInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.FirstName) == "" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите имя")
		return
	}
	var birthday any
	if in.Birthday != "" {
		parsed, err := time.Parse("2006-01-02", in.Birthday)
		if err != nil {
			fail(w, 422, "VALIDATION_ERROR", "Некорректная дата рождения")
			return
		}
		birthday = parsed
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	tag, err := a.db.Exec(r.Context(), `UPDATE customers SET first_name=$3,last_name=$4,birthday=$5,updated_at=now() WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, claims.CompanyID, claims.Subject, strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName), birthday)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Профиль не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"updated": true}})
}
func (a *api) customerHistory(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	rows, err := a.db.Query(r.Context(), `SELECT operation,amount,balance_after,description,created_at FROM bonus_ledger WHERE company_id=$1 AND customer_id=$2 ORDER BY created_at DESC LIMIT 100`, claims.CompanyID, claims.Subject)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить историю")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var operation, description string
		var amount, balance int
		var created time.Time
		if err := rows.Scan(&operation, &amount, &balance, &description, &created); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить историю")
			return
		}
		items = append(items, map[string]any{"operation": operation, "amount": amount, "balanceAfter": balance, "description": description, "createdAt": created})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить историю")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) adminDashboard(w http.ResponseWriter, r *http.Request) {
	var companies, customers, activeSubscriptions int
	var revenue float64
	err := a.db.QueryRow(r.Context(), `SELECT (SELECT count(*) FROM companies WHERE deleted_at IS NULL),(SELECT count(*) FROM customers WHERE deleted_at IS NULL),(SELECT count(*) FROM subscriptions WHERE status='active'),(SELECT coalesce(sum(amount),0) FROM subscriptions WHERE status='active')`).Scan(&companies, &customers, &activeSubscriptions, &revenue)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить платформу")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"companies": companies, "customers": customers, "activeSubscriptions": activeSubscriptions, "monthlyRevenue": revenue}})
}
func (a *api) adminCompanies(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT c.id,c.name,c.slug,c.status,c.created_at,count(distinct cu.id),coalesce(s.plan_code,'Без тарифа') FROM companies c LEFT JOIN customers cu ON cu.company_id=c.id AND cu.deleted_at IS NULL LEFT JOIN subscriptions s ON s.company_id=c.id AND s.status IN('trial','active','past_due') WHERE c.deleted_at IS NULL GROUP BY c.id,s.plan_code ORDER BY c.created_at DESC`)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить компании")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, slug, status, plan string
		var created time.Time
		var customers int
		if err := rows.Scan(&id, &name, &slug, &status, &created, &customers, &plan); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить компании")
			return
		}
		items = append(items, map[string]any{"id": id, "name": name, "slug": slug, "status": status, "createdAt": created, "customers": customers, "plan": plan})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить компании")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) adminCreateCompany(w http.ResponseWriter, r *http.Request) {
	var in adminCompanyInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Slug) == "" || !strings.Contains(in.OwnerEmail, "@") || len(in.Password) < 8 {
		fail(w, 422, "VALIDATION_ERROR", "Заполните компанию и данные владельца")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать операцию")
		return
	}
	defer tx.Rollback(r.Context())
	var company string
	err = tx.QueryRow(r.Context(), `INSERT INTO companies(name,slug) VALUES($1,$2) RETURNING id`, strings.TrimSpace(in.Name), strings.ToLower(strings.TrimSpace(in.Slug))).Scan(&company)
	var owner string
	if err == nil {
		err = tx.QueryRow(r.Context(), `INSERT INTO users(company_id,first_name,email,password_hash,role,status) VALUES($1,$2,$3,$4,'company_owner','active') RETURNING id`, company, strings.TrimSpace(in.OwnerFirstName), strings.TrimSpace(in.OwnerEmail), string(hash)).Scan(&owner)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO company_memberships(company_id,user_id,role,status) VALUES($1,$2,'owner','active')`, company, owner)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO company_modules(company_id,module_code,enabled) SELECT $1,code,is_core OR code IN('crm','loyalty') FROM modules`, company)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO subscriptions(company_id,plan_code,status,amount,current_period_ends_at) VALUES($1,'Starter','trial',0,now()+interval '14 days')`, company)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 409, "COMPANY_EXISTS", "Компания, slug или email уже существуют")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": company}})
}
