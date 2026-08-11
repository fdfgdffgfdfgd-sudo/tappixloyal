package httpapi

import (
	"net/http"
	"strings"
	"time"
)

type adminSubscriptionInput struct {
	Plan          string   `json:"plan"`
	Status        string   `json:"status"`
	Amount        float64  `json:"amount"`
	BillingPeriod string   `json:"billingPeriod"`
	PeriodEndsAt  string   `json:"periodEndsAt"`
	Modules       []string `json:"modules"`
}
type companyStatusInput struct {
	Status string `json:"status"`
}

func (a *api) adminCompanyDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var name, slug, status string
	err := a.db.QueryRow(r.Context(), `SELECT name,slug,status FROM companies WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&name, &slug, &status)
	if err != nil {
		fail(w, 404, "COMPANY_NOT_FOUND", "Компания не найдена")
		return
	}
	var subscription adminSubscriptionInput
	var periodEnd *time.Time
	err = a.db.QueryRow(r.Context(), `SELECT plan_code,status,amount,billing_period,current_period_ends_at FROM subscriptions WHERE company_id=$1 AND status IN('trial','active','past_due') ORDER BY created_at DESC LIMIT 1`, id).Scan(&subscription.Plan, &subscription.Status, &subscription.Amount, &subscription.BillingPeriod, &periodEnd)
	if err != nil {
		subscription = adminSubscriptionInput{Plan: "Starter", Status: "trial", BillingPeriod: "monthly"}
	}
	if periodEnd != nil {
		subscription.PeriodEndsAt = periodEnd.Format("2006-01-02")
	}
	rows, _ := a.db.Query(r.Context(), `SELECT module_code FROM company_modules WHERE company_id=$1 AND enabled ORDER BY module_code`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var code string
			if rows.Scan(&code) == nil {
				subscription.Modules = append(subscription.Modules, code)
			}
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": id, "name": name, "slug": slug, "status": status, "subscription": subscription}})
}
func (a *api) adminUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	var in adminSubscriptionInput
	if !decode(w, r, &in) {
		return
	}
	if in.Plan == "" || in.Amount < 0 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите тариф и корректную стоимость")
		return
	}
	validStatus := in.Status == "trial" || in.Status == "active" || in.Status == "past_due" || in.Status == "cancelled" || in.Status == "expired"
	if !validStatus {
		fail(w, 422, "VALIDATION_ERROR", "Некорректный статус подписки")
		return
	}
	if in.BillingPeriod == "" {
		in.BillingPeriod = "monthly"
	}
	planCode := normalizePlanCode(in.Plan)
	if planCode == "" {
		fail(w, 422, "VALIDATION_ERROR", "Доступны тарифы Starter, Growth и Pro")
		return
	}
	if len(in.Modules) == 0 {
		in.Modules = defaultModulesForPlan(planCode)
	}
	var period any
	if in.PeriodEndsAt != "" {
		period = in.PeriodEndsAt
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать операцию")
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `UPDATE subscriptions SET status='cancelled',cancelled_at=now(),updated_at=now() WHERE company_id=$1 AND status IN('trial','active','past_due')`, r.PathValue("id"))
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO subscriptions(company_id,plan_code,status,amount,currency,billing_period,current_period_ends_at) VALUES($1,$2,$3,$4,'KZT',$5,$6)`, r.PathValue("id"), in.Plan, in.Status, in.Amount, in.BillingPeriod, period)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE company_modules SET enabled=false,updated_at=now() WHERE company_id=$1 AND module_code NOT IN('core')`, r.PathValue("id"))
	}
	if err == nil {
		for _, code := range in.Modules {
			_, err = tx.Exec(r.Context(), `INSERT INTO company_modules(company_id,module_code,enabled) VALUES($1,$2,true) ON CONFLICT(company_id,module_code) DO UPDATE SET enabled=true,updated_at=now()`, r.PathValue("id"), code)
			if err != nil {
				break
			}
		}
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "SUBSCRIPTION_UPDATE_FAILED", "Не удалось обновить подписку")
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}

func normalizePlanCode(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "starter":
		return "starter"
	case "business", "growth":
		return "growth"
	case "enterprise", "pro":
		return "pro"
	default:
		return ""
	}
}

func defaultModulesForPlan(plan string) []string {
	modules := []string{"core", "crm", "loyalty", "reviews"}
	if plan == "growth" || plan == "pro" {
		modules = append(modules, "analytics", "website", "booking", "email", "sms", "telegram", "partnerships")
	}
	if plan == "pro" {
		modules = append(modules, "api")
	}
	return modules
}
func (a *api) adminUpdateCompanyStatus(w http.ResponseWriter, r *http.Request) {
	var in companyStatusInput
	if !decode(w, r, &in) {
		return
	}
	if in.Status != "active" && in.Status != "blocked" && in.Status != "archived" {
		fail(w, 422, "VALIDATION_ERROR", "Некорректный статус компании")
		return
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE companies SET status=$2,updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, r.PathValue("id"), in.Status)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "COMPANY_NOT_FOUND", "Компания не найдена")
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}
