package httpapi

import (
	"fmt"
	"net/http"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var companySlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type provisionCompanyInput struct {
	Company struct{ Name, LegalName, Category, Phone, WhatsApp, Instagram, Address, City, Timezone, Language, Currency, LogoURL, Slug string } `json:"company"`
	Owner   struct {
		FirstName, LastName, Phone, Email, TemporaryPassword string
		MustChangePassword                                   bool `json:"mustChangePassword"`
	} `json:"owner"`
	Subscription struct {
		Plan, Status, StartDate, EndDate                                   string
		TrialDays, CustomerLimit, StaffLimit, SmartLinkLimit, MessageLimit int
	} `json:"subscription"`
	Modules []string `json:"modules"`
}

func (a *api) adminPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT code,name,monthly_price,currency,status FROM plans_v2 ORDER BY monthly_price`)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить тарифы")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var code, name, currency, status string
		var price float64
		if rows.Scan(&code, &name, &price, &currency, &status) == nil {
			limits := map[string]any{}
			erows, err := a.db.Query(r.Context(), `SELECT code,enabled,limit_value FROM plan_entitlements WHERE plan_code=$1 ORDER BY code`, code)
			if err != nil {
				fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить лимиты тарифа")
				return
			}
			if erows != nil {
				for erows.Next() {
					var key string
					var enabled bool
					var limit *int
					if erows.Scan(&key, &enabled, &limit) == nil {
						limits[key] = map[string]any{"enabled": enabled, "limit": limit}
					}
				}
				erows.Close()
			}
			items = append(items, map[string]any{"code": code, "name": name, "monthlyPrice": price, "currency": currency, "status": status, "entitlements": limits})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) adminProvisionCompany(w http.ResponseWriter, r *http.Request) {
	var in provisionCompanyInput
	if !decode(w, r, &in) {
		return
	}
	in.Company.Name = strings.TrimSpace(in.Company.Name)
	in.Company.Slug = strings.ToLower(strings.TrimSpace(in.Company.Slug))
	in.Owner.Email = strings.ToLower(strings.TrimSpace(in.Owner.Email))
	if in.Company.Name == "" || !companySlugPattern.MatchString(in.Company.Slug) || in.Company.Category == "" || !strings.Contains(in.Owner.Email, "@") || in.Owner.FirstName == "" || len(in.Owner.TemporaryPassword) < 8 || in.Subscription.Plan == "" {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте компанию, владельца и тариф")
		return
	}
	if in.Company.Timezone == "" {
		in.Company.Timezone = "Asia/Almaty"
	}
	if in.Company.Language == "" {
		in.Company.Language = "ru"
	}
	if in.Company.Currency == "" {
		in.Company.Currency = "KZT"
	}
	if in.Subscription.Status == "" {
		in.Subscription.Status = "trial"
	}
	if in.Subscription.Status != "trial" && in.Subscription.Status != "active" {
		fail(w, 422, "VALIDATION_ERROR", "Статус должен быть trial или active")
		return
	}
	var starts, ends time.Time
	var err error
	if in.Subscription.StartDate == "" {
		starts = time.Now()
	} else {
		starts, err = time.Parse("2006-01-02", in.Subscription.StartDate)
	}
	if err != nil {
		fail(w, 422, "VALIDATION_ERROR", "Некорректная дата начала")
		return
	}
	if in.Subscription.EndDate != "" {
		ends, err = time.Parse("2006-01-02", in.Subscription.EndDate)
	} else if in.Subscription.TrialDays > 0 {
		ends = starts.AddDate(0, 0, in.Subscription.TrialDays)
	} else {
		ends = starts.AddDate(0, 1, 0)
	}
	if err != nil || !ends.After(starts) {
		fail(w, 422, "VALIDATION_ERROR", "Дата окончания должна быть позже даты начала")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Owner.TemporaryPassword), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "PASSWORD_ERROR", "Не удалось обработать пароль")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать создание")
		return
	}
	defer tx.Rollback(r.Context())
	var company, owner, branch, device, deviceToken, smartLink string
	err = tx.QueryRow(r.Context(), `INSERT INTO companies(name,legal_name,category,slug,logo_url,phone,whatsapp,instagram,address,city,timezone,language,currency) VALUES($1,nullif($2,''),$3,$4,nullif($5,''),nullif($6,''),nullif($7,''),nullif($8,''),nullif($9,''),nullif($10,''),$11,$12,$13) RETURNING id`, in.Company.Name, in.Company.LegalName, in.Company.Category, in.Company.Slug, in.Company.LogoURL, in.Company.Phone, in.Company.WhatsApp, in.Company.Instagram, in.Company.Address, in.Company.City, in.Company.Timezone, in.Company.Language, in.Company.Currency).Scan(&company)
	if err == nil {
		err = tx.QueryRow(r.Context(), `INSERT INTO users(company_id,first_name,last_name,phone,email,password_hash,role,status,must_change_password) VALUES($1,$2,$3,nullif($4,''),$5,$6,'company_owner','active',$7) RETURNING id`, company, strings.TrimSpace(in.Owner.FirstName), strings.TrimSpace(in.Owner.LastName), in.Owner.Phone, in.Owner.Email, string(hash), in.Owner.MustChangePassword).Scan(&owner)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO company_memberships(company_id,user_id,role,status) VALUES($1,$2,'owner','active')`, company, owner)
	}
	var price float64
	if err == nil {
		err = tx.QueryRow(r.Context(), `SELECT monthly_price FROM plans_v2 WHERE code=$1 AND status='active'`, in.Subscription.Plan).Scan(&price)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO subscriptions(company_id,plan_code,status,amount,currency,billing_period,starts_at,current_period_ends_at) VALUES($1,$2,$3,$4,$5,'monthly',$6,$7)`, company, in.Subscription.Plan, in.Subscription.Status, price, in.Company.Currency, starts, ends)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO company_modules(company_id,module_code,enabled) SELECT $1,code,is_core OR code=ANY($2::varchar[]) FROM modules`, company, in.Modules)
	}
	limits := map[string]int{"customers": in.Subscription.CustomerLimit, "staff": in.Subscription.StaffLimit, "smart_links": in.Subscription.SmartLinkLimit, "messages_monthly": in.Subscription.MessageLimit}
	for code, value := range limits {
		if err != nil {
			break
		}
		if value > 0 {
			_, err = tx.Exec(r.Context(), `INSERT INTO subscription_overrides(company_id,entitlement_code,enabled,limit_value,reason,created_by) VALUES($1,$2,true,$3,'Установлено при создании компании',$4)`, company, code, value, identitySubject(r))
		}
	}
	address := strings.TrimSpace(in.Company.Address)
	if address == "" {
		address = "Основной филиал"
	}
	if err == nil {
		err = tx.QueryRow(r.Context(), `INSERT INTO branches(company_id,name,address,phone) VALUES($1,'Основной филиал',$2,$3) RETURNING id`, company, address, in.Company.Phone).Scan(&branch)
	}
	if err == nil {
		err = tx.QueryRow(r.Context(), `INSERT INTO devices(company_id,branch_id,kind,name,destination) VALUES($1,$2,'qr','Основная QR-точка','join') RETURNING id,token`, company, branch).Scan(&device, &deviceToken)
	}
	if err == nil {
		err = tx.QueryRow(r.Context(), `INSERT INTO smart_links(company_id,branch_id,device_id,slug,source) VALUES($1,$2,$3,'main','qr') RETURNING id`, company, branch, device).Scan(&smartLink)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO company_settings(company_id,branding) VALUES($1,jsonb_build_object('guestPortal',jsonb_build_object('welcomeTitle','Добро пожаловать в '||$2||'!','primaryColor','#2563EB','logoUrl',$3)))`, company, in.Company.Name, in.Company.LogoURL)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data) VALUES($1,$2,'platform.company.provisioned','company',$1,$3,jsonb_build_object('plan',$4,'ownerId',$5,'smartLinkId',$6))`, company, identitySubject(r), r.Header.Get("X-Request-ID"), in.Subscription.Plan, owner, smartLink)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 409, "PROVISIONING_FAILED", "Компания, slug или email уже существуют либо параметры тарифа некорректны")
		return
	}
	guestURL := strings.TrimRight(envOr("APP_URL", "http://localhost:8088"), "/") + "/join/" + deviceToken
	go a.sendProvisioningInvite(in.Owner.Email, in.Owner.FirstName, in.Company.Name, guestURL)
	write(w, 201, envelope{Success: true, Data: map[string]any{"companyId": company, "ownerId": owner, "branchId": branch, "deviceId": device, "smartLinkId": smartLink, "guestUrl": guestURL, "subscriptionEndsAt": ends, "plan": in.Subscription.Plan}})
}

func identitySubject(r *http.Request) string {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	return claims.Subject
}

func (a *api) sendProvisioningInvite(email, name, company, guestURL string) {
	from := envValue("SMTP_FROM", "Tappix <noreply@tappix.kz>")
	body := fmt.Sprintf("Здравствуйте, %s!\n\nДля вас создано рабочее пространство %s в Tappix. Войдите с временным паролем, полученным от администратора платформы.\n\nГостевая ссылка: %s\n", name, company, guestURL)
	message := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Ваше рабочее пространство Tappix готово\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", from, email, body))
	_ = smtp.SendMail(envValue("SMTP_HOST", "mailpit")+":"+envValue("SMTP_PORT", "1025"), nil, fromAddress(from), []string{email}, message)
}
