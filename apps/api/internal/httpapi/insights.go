package httpapi

import (
	"fmt"
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

type analyticsDateRange struct {
	Key                            string
	Start, End, PreviousStart, PreviousEnd time.Time
}

func resolveAnalyticsRange(now time.Time, location *time.Location, period, from, to string) (analyticsDateRange, error) {
	local := now.In(location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	end := today.AddDate(0, 0, 1)
	start := today.AddDate(0, 0, -29)
	key := period
	switch period {
	case "today":
		start = today
	case "yesterday":
		start, end = today.AddDate(0, 0, -1), today
	case "week":
		start = today.AddDate(0, 0, -6)
	case "", "month":
		key = "month"
	case "quarter":
		start = today.AddDate(0, 0, -89)
	case "this_month":
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	case "previous_month":
		end = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
		start = end.AddDate(0, -1, 0)
	case "custom":
		var err error
		start, err = time.ParseInLocation("2006-01-02", from, location)
		if err != nil {
			return analyticsDateRange{}, fmt.Errorf("invalid start date")
		}
		last, parseErr := time.ParseInLocation("2006-01-02", to, location)
		if parseErr != nil || last.Before(start) || last.Sub(start) > 366*24*time.Hour {
			return analyticsDateRange{}, fmt.Errorf("invalid end date")
		}
		end = last.AddDate(0, 0, 1)
	default:
		return analyticsDateRange{}, fmt.Errorf("unsupported period")
	}
	duration := end.Sub(start)
	return analyticsDateRange{Key: key, Start: start, End: end, PreviousStart: start.Add(-duration), PreviousEnd: start}, nil
}

func (a *api) analytics(w http.ResponseWriter, r *http.Request) {
	tenant := companyID(r)
	period := r.URL.Query().Get("period")
	timezone := "Asia/Almaty"
	_ = a.db.QueryRow(r.Context(), `SELECT timezone FROM companies WHERE id=$1`, tenant).Scan(&timezone)
	location, locationErr := time.LoadLocation(timezone)
	if locationErr != nil {
		location = time.UTC
	}
	rangeValue, rangeErr := resolveAnalyticsRange(time.Now(), location, period, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if rangeErr != nil {
		fail(w, 422, "INVALID_ANALYTICS_PERIOD", "Проверьте выбранный период")
		return
	}
	branch := strings.TrimSpace(r.URL.Query().Get("branchId"))
	// Do not let malformed user input reach a UUID cast in the query.
	if len(branch) != 36 || strings.Count(branch, "-") != 4 {
		branch = ""
	}
	// Always bind a string (including the empty string) so pgx/PostgreSQL can
	// infer one stable type for the optional branch parameter. Passing an
	// untyped nil here makes inference driver/version dependent.
	branchArg := branch
	days := int(rangeValue.End.Sub(rangeValue.Start).Hours() / 24)
	rows, err := a.db.Query(r.Context(), `SELECT d::date,
		 (SELECT count(*) FROM customers c WHERE c.company_id=$1 AND c.created_at::date=d::date AND c.deleted_at IS NULL AND ($4::text = '' OR EXISTS(SELECT 1 FROM visits vf WHERE vf.company_id=c.company_id AND vf.customer_id=c.id AND vf.branch_id=CAST(NULLIF($4::text,'') AS uuid)))),
		 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at::date=d::date AND ($4::text = '' OR v.branch_id=CAST(NULLIF($4::text,'') AS uuid))),
		 (SELECT coalesce(sum(v.points_added),0) FROM visits v WHERE v.company_id=$1 AND v.created_at::date=d::date AND ($4::text = '' OR v.branch_id=CAST(NULLIF($4::text,'') AS uuid))),
		 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at::date=d::date AND ($4::text = '' OR v.branch_id=CAST(NULLIF($4::text,'') AS uuid)) AND v.created_at=(SELECT min(v2.created_at) FROM visits v2 WHERE v2.company_id=$1 AND v2.customer_id=v.customer_id)),
		 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at::date=d::date AND ($4::text = '' OR v.branch_id=CAST(NULLIF($4::text,'') AS uuid)) AND v.created_at>(SELECT min(v2.created_at) FROM visits v2 WHERE v2.company_id=$1 AND v2.customer_id=v.customer_id))
		 FROM generate_series($2::timestamptz,($3::timestamptz-interval '1 second'),interval '1 day') d ORDER BY d`, tenant, rangeValue.Start, rangeValue.End, branchArg)
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
		 (SELECT count(*) FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND ($3::text = '' OR EXISTS(SELECT 1 FROM visits vf WHERE vf.company_id=c.company_id AND vf.customer_id=c.id AND vf.branch_id=CAST(NULLIF($3::text,'') AS uuid))))),
		 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at>=current_date-make_interval(days=>$2-1) AND ($3::text = '' OR v.branch_id=CAST(NULLIF($3::text,'') AS uuid))),
		 (SELECT coalesce(sum(amount),0) FROM bonus_ledger b WHERE b.company_id=$1 AND b.operation='credit' AND b.created_at>=current_date-make_interval(days=>$2-1) AND ($3::text = '' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=b.company_id AND v.customer_id=b.customer_id AND v.branch_id=CAST(NULLIF($3::text,'') AS uuid)))),
		 (SELECT coalesce(sum(amount),0) FROM bonus_ledger b WHERE b.company_id=$1 AND b.operation='debit' AND b.created_at>=current_date-make_interval(days=>$2-1) AND ($3::text = '' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=b.company_id AND v.customer_id=b.customer_id AND v.branch_id=CAST(NULLIF($3::text,'') AS uuid)))),
		 (SELECT count(DISTINCT customer_id) FROM visits v WHERE v.company_id=$1 AND v.created_at>=current_date-make_interval(days=>$2-1) AND ($3::text = '' OR v.branch_id=CAST(NULLIF($3::text,'') AS uuid))),
		 (SELECT count(*) FROM (SELECT customer_id FROM visits v WHERE v.company_id=$1 AND v.created_at>=current_date-make_interval(days=>$2-1) AND ($3::text = '' OR v.branch_id=CAST(NULLIF($3::text,'') AS uuid)) GROUP BY customer_id HAVING count(*)>=2) q),
		 (SELECT count(*) FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND c.created_at>=current_date-make_interval(days=>$2-1) AND ($3::text = '' OR EXISTS(SELECT 1 FROM visits vf WHERE vf.company_id=c.company_id AND vf.customer_id=c.id AND vf.branch_id=CAST(NULLIF($3::text,'') AS uuid)))))`, tenant, days, branchArg).Scan(&totalCustomers, &periodVisits, &pointsIssued, &pointsRedeemed, &active, &repeatActive, &newCustomers)
	var previousVisits, previousActive, previousNew, previousIssued int
	_ = a.db.QueryRow(r.Context(), `SELECT
	 (SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at>=current_date-make_interval(days=>$2*2-1) AND v.created_at<current_date-make_interval(days=>$2-1) AND ($3::text='' OR v.branch_id=nullif($3::text,'')::uuid)),
	 (SELECT count(DISTINCT customer_id) FROM visits v WHERE v.company_id=$1 AND v.created_at>=current_date-make_interval(days=>$2*2-1) AND v.created_at<current_date-make_interval(days=>$2-1) AND ($3::text='' OR v.branch_id=nullif($3::text,'')::uuid)),
	 (SELECT count(*) FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND c.created_at>=current_date-make_interval(days=>$2*2-1) AND c.created_at<current_date-make_interval(days=>$2-1) AND ($3::text='' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=c.company_id AND v.customer_id=c.id AND v.branch_id=nullif($3::text,'')::uuid))),
	 (SELECT coalesce(sum(amount),0) FROM bonus_ledger b WHERE b.company_id=$1 AND b.operation='credit' AND b.created_at>=current_date-make_interval(days=>$2*2-1) AND b.created_at<current_date-make_interval(days=>$2-1) AND ($3::text='' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=b.company_id AND v.customer_id=b.customer_id AND v.branch_id=nullif($3::text,'')::uuid)))`, tenant, days, branchArg).Scan(&previousVisits, &previousActive, &previousNew, &previousIssued)
	var returning, frequent, loyal, atRisk, outstanding int
	_ = a.db.QueryRow(r.Context(), `SELECT
	 count(*) FILTER(WHERE EXISTS(SELECT 1 FROM visits v WHERE v.company_id=$1 AND v.customer_id=c.id AND v.created_at>=now()-make_interval(days=>$2) AND ($3::text='' OR v.branch_id=nullif($3::text,'')::uuid))),
	 count(*) FILTER(WHERE c.total_visits>=2),count(*) FILTER(WHERE c.total_visits>=5),count(*) FILTER(WHERE c.total_visits>=10),
	 count(*) FILTER(WHERE c.total_visits>0 AND NOT EXISTS(SELECT 1 FROM visits v WHERE v.company_id=$1 AND v.customer_id=c.id AND v.created_at>=now()-interval '45 days' AND ($3::text='' OR v.branch_id=nullif($3::text,'')::uuid))),
	 count(*) FILTER(WHERE c.created_at>=current_date-make_interval(days=>$2))
	 FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND ($3::text='' OR EXISTS(SELECT 1 FROM visits vb WHERE vb.company_id=c.company_id AND vb.customer_id=c.id AND vb.branch_id=nullif($3::text,'')::uuid))`, tenant, days, branchArg).Scan(&active, &returning, &frequent, &loyal, &atRisk, &newCustomers)
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
	topRows, err := a.db.Query(r.Context(), `SELECT c.id,c.first_name,c.last_name,c.total_visits,c.total_points,c.level FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND ($2::text='' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=c.company_id AND v.customer_id=c.id AND v.branch_id=nullif($2::text,'')::uuid)) ORDER BY c.total_visits DESC,c.total_points DESC LIMIT 5`, tenant, branchArg)
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
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce((SELECT extract(hour from created_at)::int FROM visits WHERE company_id=$1 AND created_at>=current_date-make_interval(days=>$2-1) AND ($3::text='' OR branch_id=nullif($3::text,'')::uuid) GROUP BY 1 ORDER BY count(*) DESC LIMIT 1),0)`, tenant, days, branchArg).Scan(&peakHour)
	write(w, 200, envelope{Success: true, Data: map[string]any{
		"period": period, "days": days, "series": series,
		"totals":       map[string]int{"customers": totalCustomers, "visits": periodVisits, "pointsIssued": pointsIssued, "pointsRedeemed": pointsRedeemed, "outstanding": outstanding},
		"previous":     map[string]int{"visits": previousVisits, "active": previousActive, "new": previousNew, "pointsIssued": previousIssued},
		"comparisonAvailable": previousVisits > 0 || previousActive > 0 || previousNew > 0 || previousIssued > 0,
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
