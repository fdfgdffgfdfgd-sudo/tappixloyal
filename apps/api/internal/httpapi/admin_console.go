package httpapi

import (
	"net/http"
	"strings"
	"time"
)

type adminPlanUpdate struct {
	Name         string  `json:"name"`
	MonthlyPrice float64 `json:"monthlyPrice"`
	Status       string  `json:"status"`
}

func (a *api) adminUpdatePlan(w http.ResponseWriter, r *http.Request) {
	var in adminPlanUpdate
	if !decode(w, r, &in) {
		return
	}
	code := strings.ToLower(strings.TrimSpace(r.PathValue("code")))
	if code == "" || strings.TrimSpace(in.Name) == "" || in.MonthlyPrice < 0 || (in.Status != "active" && in.Status != "archived") {
		fail(w, 422, "VALIDATION_ERROR", "Укажите название, цену и корректный статус тарифа")
		return
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE plans_v2 SET name=$2,monthly_price=$3,status=$4,updated_at=now() WHERE code=$1`, code, strings.TrimSpace(in.Name), in.MonthlyPrice, in.Status)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "PLAN_NOT_FOUND", "Тариф не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"code": code, "name": in.Name, "monthlyPrice": in.MonthlyPrice, "status": in.Status}})
}

func (a *api) adminUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT u.id,u.first_name,u.last_name,u.email,u.role,u.status,coalesce(c.name,'Tappix Platform'),u.created_at FROM users u LEFT JOIN companies c ON c.id=u.company_id WHERE u.deleted_at IS NULL ORDER BY u.created_at DESC LIMIT 500`)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить пользователей")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, first, last, email, role, status, company string
		var created time.Time
		if rows.Scan(&id, &first, &last, &email, &role, &status, &company, &created) == nil {
			items = append(items, map[string]any{"id": id, "firstName": first, "lastName": last, "email": email, "role": role, "status": status, "company": company, "createdAt": created})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) adminPlatformAnalytics(w http.ResponseWriter, r *http.Request) {
	var companies, customers, visits, frequent, loyal, atRisk, newMonth int
	err := a.db.QueryRow(r.Context(), `SELECT
	 (SELECT count(*) FROM companies WHERE deleted_at IS NULL),
	 (SELECT count(*) FROM customers WHERE deleted_at IS NULL),
	 (SELECT count(*) FROM visits),
	 (SELECT count(*) FROM customers WHERE deleted_at IS NULL AND total_visits>=5),
	 (SELECT count(*) FROM customers WHERE deleted_at IS NULL AND total_visits>=10),
	 (SELECT count(*) FROM customers c WHERE c.deleted_at IS NULL AND c.total_visits>0 AND NOT EXISTS(SELECT 1 FROM visits v WHERE v.customer_id=c.id AND v.created_at>now()-interval '45 days')),
	 (SELECT count(*) FROM customers WHERE deleted_at IS NULL AND created_at>=date_trunc('month',now()))`).Scan(&companies, &customers, &visits, &frequent, &loyal, &atRisk, &newMonth)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось собрать аналитику платформы")
		return
	}
	companyRows, err := a.db.Query(r.Context(), `SELECT co.id,co.name,co.slug,count(distinct c.id),count(distinct v.id),count(distinct c.id) FILTER (WHERE c.total_visits>=2),count(distinct c.id) FILTER (WHERE c.total_visits>0 AND NOT EXISTS(SELECT 1 FROM visits recent WHERE recent.customer_id=c.id AND recent.created_at>now()-interval '45 days')) FROM companies co LEFT JOIN customers c ON c.company_id=co.id AND c.deleted_at IS NULL LEFT JOIN visits v ON v.company_id=co.id WHERE co.deleted_at IS NULL GROUP BY co.id ORDER BY count(distinct c.id) DESC`)
	if err != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить аналитику платформы")
		return
	}
	breakdown := []map[string]any{}
	if companyRows != nil {
		defer companyRows.Close()
		for companyRows.Next() {
			var id, name, slug string
			var customerCount, visitCount, returningCount, riskCount int
			if companyRows.Scan(&id, &name, &slug, &customerCount, &visitCount, &returningCount, &riskCount) == nil {
				breakdown = append(breakdown, map[string]any{"id": id, "name": name, "slug": slug, "customers": customerCount, "visits": visitCount, "returning": returningCount, "atRisk": riskCount})
			}
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"companies": companies, "customers": customers, "visits": visits, "frequent": frequent, "loyal": loyal, "atRisk": atRisk, "newThisMonth": newMonth, "retentionRate": percent(frequent, customers), "averageVisits": average(visits, customers), "companyBreakdown": breakdown}})
}

func percent(part, total int) int {
	if total == 0 {
		return 0
	}
	return int(float64(part) / float64(total) * 100)
}

func average(total, count int) float64 {
	if count == 0 {
		return 0
	}
	return float64(total) / float64(count)
}

func (a *api) adminCustomers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT c.id,co.id,co.name,c.first_name,c.last_name,c.phone,c.total_points,c.total_visits,c.level,c.created_at,(SELECT max(v.created_at) FROM visits v WHERE v.customer_id=c.id) FROM customers c JOIN companies co ON co.id=c.company_id WHERE c.deleted_at IS NULL ORDER BY c.total_visits DESC,c.created_at DESC LIMIT 1000`)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить клиентов платформы")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, companyID, company, first, last, phone, level string
		var points, visits int
		var created time.Time
		var lastVisit *time.Time
		if rows.Scan(&id, &companyID, &company, &first, &last, &phone, &points, &visits, &level, &created, &lastVisit) == nil {
			segment := "Новый"
			if visits >= 10 {
				segment = "Постоянный"
			} else if visits >= 5 {
				segment = "Частый"
			} else if visits >= 2 {
				segment = "Возвращается"
			}
			if visits > 0 && (lastVisit == nil || lastVisit.Before(time.Now().AddDate(0, 0, -45))) {
				segment = "Риск ухода"
			}
			items = append(items, map[string]any{"id": id, "companyId": companyID, "company": company, "firstName": first, "lastName": last, "phone": phone, "points": points, "visits": visits, "level": level, "segment": segment, "createdAt": created, "lastVisitAt": lastVisit})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
