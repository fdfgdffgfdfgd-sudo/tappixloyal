package httpapi

import (
	"net/http"
	"strings"
	"time"
)

type companySettingsInput struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Address  string `json:"address"`
	Timezone string `json:"timezone"`
	Language string `json:"language"`
}
type reviewSettingsInput struct {
	GISURL            string  `json:"gisUrl"`
	GoogleURL         string  `json:"googleUrl"`
	YandexURL         string  `json:"yandexUrl"`
	RedirectThreshold float64 `json:"redirectThreshold"`
	Enabled           bool    `json:"enabled"`
}

func (a *api) analytics(w http.ResponseWriter, r *http.Request) {
	tenant := companyID(r)
	period := r.URL.Query().Get("period")
	branch := strings.TrimSpace(r.URL.Query().Get("branchId"))
	// Do not let malformed user input reach a UUID cast in the query.
	if len(branch) != 36 || strings.Count(branch, "-") != 4 {
		branch = ""
	}
	days := 30
	if period == "week" {
		days = 7
	} else if period == "quarter" {
		days = 90
	}
	rows, err := a.db.Query(r.Context(), `SELECT d::date,
		 (SELECT count(*) FROM customers c WHERE c.company_id=$1 AND c.created_at::date=d::date AND c.deleted_at IS NULL AND ($3='' OR EXISTS(SELECT 1 FROM visits vf WHERE vf.company_id=c.company_id AND vf.customer_id=c.id AND vf.branch_id=nullif($3,'')::uuid)),
		 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at::date=d::date AND ($3='' OR v.branch_id=nullif($3,'')::uuid)),
		 (SELECT coalesce(sum(v.points_added),0) FROM visits v WHERE v.company_id=$1 AND v.created_at::date=d::date AND ($3='' OR v.branch_id=nullif($3,'')::uuid)),
		 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at::date=d::date AND ($3='' OR v.branch_id=nullif($3,'')::uuid) AND v.created_at=(SELECT min(v2.created_at) FROM visits v2 WHERE v2.company_id=$1 AND v2.customer_id=v.customer_id)),
		 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at::date=d::date AND ($3='' OR v.branch_id=nullif($3,'')::uuid) AND v.created_at>(SELECT min(v2.created_at) FROM visits v2 WHERE v2.company_id=$1 AND v2.customer_id=v.customer_id))
		 FROM generate_series(current_date-$2::int,current_date,interval '1 day') d ORDER BY d`, tenant, days-1, branch)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить аналитику")
		return
	}
	defer rows.Close()
	series := []map[string]any{}
	for rows.Next() {
		var date time.Time
		var customers, visits, points, firstVisits, repeatVisits int
		if err := rows.Scan(&date, &customers, &visits, &points, &firstVisits, &repeatVisits); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить аналитику")
			return
		}
		series = append(series, map[string]any{"date": date, "customers": customers, "visits": visits, "points": points, "firstVisits": firstVisits, "repeatVisits": repeatVisits})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить аналитику")
		return
	}
	var totalCustomers, periodVisits, pointsIssued, pointsRedeemed, active, repeatActive, newCustomers int
	_ = a.db.QueryRow(r.Context(), `SELECT
	 (SELECT count(*) FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND ($3='' OR EXISTS(SELECT 1 FROM visits vf WHERE vf.company_id=c.company_id AND vf.customer_id=c.id AND vf.branch_id=nullif($3,'')::uuid))),
	 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at>=current_date-make_interval(days=>$2-1) AND ($3='' OR v.branch_id=nullif($3,'')::uuid)),
	 (SELECT coalesce(sum(amount),0) FROM bonus_ledger b WHERE b.company_id=$1 AND b.operation='credit' AND b.created_at>=current_date-make_interval(days=>$2-1) AND ($3='' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=b.company_id AND v.customer_id=b.customer_id AND v.branch_id=nullif($3,'')::uuid))),
	 (SELECT coalesce(sum(amount),0) FROM bonus_ledger b WHERE b.company_id=$1 AND b.operation='debit' AND b.created_at>=current_date-make_interval(days=>$2-1) AND ($3='' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=b.company_id AND v.customer_id=b.customer_id AND v.branch_id=nullif($3,'')::uuid))),
	 (SELECT count(DISTINCT customer_id) FROM visits v WHERE v.company_id=$1 AND v.created_at>=current_date-make_interval(days=>$2-1) AND ($3='' OR v.branch_id=nullif($3,'')::uuid)),
	 (SELECT count(*) FROM (SELECT customer_id FROM visits v WHERE v.company_id=$1 AND v.created_at>=current_date-make_interval(days=>$2-1) AND ($3='' OR v.branch_id=nullif($3,'')::uuid) GROUP BY customer_id HAVING count(*)>=2) q),
	 (SELECT count(*) FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND c.created_at>=current_date-make_interval(days=>$2-1) AND ($3='' OR EXISTS(SELECT 1 FROM visits vf WHERE vf.company_id=c.company_id AND vf.customer_id=c.id AND vf.branch_id=nullif($3,'')::uuid)))`, tenant, days, branch).Scan(&totalCustomers, &periodVisits, &pointsIssued, &pointsRedeemed, &active, &repeatActive, &newCustomers)
	var previousVisits, previousActive, previousNew, previousIssued int
	_ = a.db.QueryRow(r.Context(), `SELECT
	 (SELECT count(*) FROM visits WHERE company_id=$1 AND created_at>=current_date-make_interval(days=>$2*2-1) AND created_at<current_date-make_interval(days=>$2-1)),
	 (SELECT count(DISTINCT customer_id) FROM visits WHERE company_id=$1 AND created_at>=current_date-make_interval(days=>$2*2-1) AND created_at<current_date-make_interval(days=>$2-1)),
	 (SELECT count(*) FROM customers WHERE company_id=$1 AND deleted_at IS NULL AND created_at>=current_date-make_interval(days=>$2*2-1) AND created_at<current_date-make_interval(days=>$2-1)),
	 (SELECT coalesce(sum(amount),0) FROM bonus_ledger WHERE company_id=$1 AND operation='credit' AND created_at>=current_date-make_interval(days=>$2*2-1) AND created_at<current_date-make_interval(days=>$2-1))`, tenant, days).Scan(&previousVisits, &previousActive, &previousNew, &previousIssued)
	var returning, frequent, loyal, atRisk, outstanding int
	_ = a.db.QueryRow(r.Context(), `SELECT
	 count(*) FILTER(WHERE EXISTS(SELECT 1 FROM visits v WHERE v.company_id=$1 AND v.customer_id=c.id AND v.created_at>=now()-make_interval(days=>$2))),
	 count(*) FILTER(WHERE c.total_visits>=2),count(*) FILTER(WHERE c.total_visits>=5),count(*) FILTER(WHERE c.total_visits>=10),
	 count(*) FILTER(WHERE c.total_visits>0 AND NOT EXISTS(SELECT 1 FROM visits v WHERE v.company_id=$1 AND v.customer_id=c.id AND v.created_at>=now()-interval '45 days')),
	 count(*) FILTER(WHERE c.created_at>=current_date-make_interval(days=>$2))
	 FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL`, tenant, days).Scan(&active, &returning, &frequent, &loyal, &atRisk, &newCustomers)
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce(sum(total_points),0) FROM customers WHERE company_id=$1 AND deleted_at IS NULL`, tenant).Scan(&outstanding)
	retention := 0.0
	if active > 0 {
		retention = float64(repeatActive) * 100 / float64(active)
	}
	averageVisits := 0.0
	if active > 0 {
		averageVisits = float64(periodVisits) / float64(active)
	}
	top := []map[string]any{}
	topRows, err := a.db.Query(r.Context(), `SELECT id,first_name,last_name,total_visits,total_points,level FROM customers WHERE company_id=$1 AND deleted_at IS NULL ORDER BY total_visits DESC,total_points DESC LIMIT 5`, tenant)
	if err != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить топ клиентов")
		return
	}
	if topRows != nil {
		defer topRows.Close()
		for topRows.Next() {
			var id, first, last, level string
			var visits, points int
			if err := topRows.Scan(&id, &first, &last, &visits, &points, &level); err != nil {
				topRows.Close()
				fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить топ клиентов")
				return
			}
			top = append(top, map[string]any{"id": id, "name": strings.TrimSpace(first + " " + last), "visits": visits, "points": points, "level": level})
		}
		topRows.Close()
		if topRows.Err() != nil {
			fail(w, 500, "INTERNAL_ERROR", "Не удалось загрузить топ клиентов")
			return
		}
	}
	var peakHour int
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce((SELECT extract(hour from created_at)::int FROM visits WHERE company_id=$1 GROUP BY 1 ORDER BY count(*) DESC LIMIT 1),0)`, tenant).Scan(&peakHour)
	write(w, 200, envelope{Success: true, Data: map[string]any{
		"period": period, "days": days, "series": series,
		"totals":       map[string]int{"customers": totalCustomers, "visits": periodVisits, "pointsIssued": pointsIssued, "pointsRedeemed": pointsRedeemed, "outstanding": outstanding},
		"previous":     map[string]int{"visits": previousVisits, "active": previousActive, "new": previousNew, "pointsIssued": previousIssued},
		"audience":     map[string]any{"active": active, "returning": returning, "repeatActive": repeatActive, "frequent": frequent, "loyal": loyal, "atRisk": atRisk, "new": newCustomers, "retentionRate": retention, "averageVisits": averageVisits},
		"topCustomers": top, "peakHour": peakHour,
	}})
}

func (a *api) auditMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if r.Method == "GET" || r.Method == "OPTIONS" {
			return
		}
		claims, _ := r.Context().Value(identityKey).(tokenClaims)
		if claims.Subject == "" {
			return
		}
		var company any
		if claims.CompanyID != "" {
			company = claims.CompanyID
		}
		requestID := w.Header().Get("X-Request-ID")
		host := clientIP(r)
		// Guest identities are customer UUIDs, not user UUIDs. Preserve the
		// tenant-scoped request audit without violating the users FK or
		// misrepresenting a customer as a staff actor.
		_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,request_id,ip,user_agent)
			VALUES($1,(SELECT id FROM users WHERE id=nullif($2,'')::uuid),$3,$4,$5,$6,$7)`, company, claims.Subject, strings.ToLower(r.Method)+" "+r.URL.Path, "http_request", requestID, host, r.UserAgent())
	})
}
func (a *api) auditList(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	query := `SELECT a.id,a.action,a.entity_type,coalesce(a.request_id,''),a.ip::text,a.created_at,coalesce(u.first_name,'System'),coalesce(c.name,'Tappix') FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_id LEFT JOIN companies c ON c.id=a.company_id`
	args := []any{}
	if claims.Role != "super_admin" {
		query += ` WHERE a.company_id=$1`
		args = append(args, claims.CompanyID)
	}
	query += ` ORDER BY a.created_at DESC LIMIT 100`
	rows, err := a.db.Query(r.Context(), query, args...)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить аудит")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var action, entity, requestID, user, company string
		var ip string
		var created time.Time
		if err := rows.Scan(&id, &action, &entity, &requestID, &ip, &created, &user, &company); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить аудит")
			return
		}
		items = append(items, map[string]any{"id": id, "action": action, "entityType": entity, "requestId": requestID, "ip": ip, "createdAt": created, "user": user, "company": company})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить аудит")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) getCompanySettings(w http.ResponseWriter, r *http.Request) {
	var data companySettingsInput
	err := a.db.QueryRow(r.Context(), `SELECT name,coalesce(phone,''),coalesce(email,''),coalesce(address,''),timezone,language FROM companies WHERE id=$1`, companyID(r)).Scan(&data.Name, &data.Phone, &data.Email, &data.Address, &data.Timezone, &data.Language)
	if err != nil {
		fail(w, 404, "COMPANY_NOT_FOUND", "Компания не найдена")
		return
	}
	write(w, 200, envelope{Success: true, Data: data})
}
func (a *api) updateCompanySettings(w http.ResponseWriter, r *http.Request) {
	var in companySettingsInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		fail(w, 422, "VALIDATION_ERROR", "Название обязательно")
		return
	}
	if in.Timezone == "" {
		in.Timezone = "Asia/Almaty"
	}
	if in.Language == "" {
		in.Language = "ru"
	}
	_, err := a.db.Exec(r.Context(), `UPDATE companies SET name=$2,phone=$3,email=$4,address=$5,timezone=$6,language=$7,updated_at=now() WHERE id=$1`, companyID(r), strings.TrimSpace(in.Name), in.Phone, in.Email, in.Address, in.Timezone, in.Language)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сохранить компанию")
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}
func (a *api) getReviewSettings(w http.ResponseWriter, r *http.Request) {
	var data reviewSettingsInput
	err := a.db.QueryRow(r.Context(), `SELECT coalesce(gis_url,''),coalesce(google_url,''),coalesce(yandex_url,''),redirect_threshold,enabled FROM review_settings WHERE company_id=$1`, companyID(r)).Scan(&data.GISURL, &data.GoogleURL, &data.YandexURL, &data.RedirectThreshold, &data.Enabled)
	if err != nil {
		data.RedirectThreshold = 4
	}
	write(w, 200, envelope{Success: true, Data: data})
}
func (a *api) updateReviewSettings(w http.ResponseWriter, r *http.Request) {
	var in reviewSettingsInput
	if !decode(w, r, &in) {
		return
	}
	if in.RedirectThreshold < 1 || in.RedirectThreshold > 5 {
		fail(w, 422, "VALIDATION_ERROR", "Порог должен быть от 1 до 5")
		return
	}
	_, err := a.db.Exec(r.Context(), `INSERT INTO review_settings(company_id,gis_url,google_url,yandex_url,redirect_threshold,enabled) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(company_id) DO UPDATE SET gis_url=excluded.gis_url,google_url=excluded.google_url,yandex_url=excluded.yandex_url,redirect_threshold=excluded.redirect_threshold,enabled=excluded.enabled,updated_at=now()`, companyID(r), in.GISURL, in.GoogleURL, in.YandexURL, in.RedirectThreshold, in.Enabled)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сохранить отзывы")
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}
