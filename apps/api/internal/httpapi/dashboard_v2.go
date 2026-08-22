package httpapi

import (
	"net/http"
	"strings"
	"time"
)

func dashboardRange(value string, now time.Time) (time.Time, time.Time, string) {
	end := now
	start := now.AddDate(0, 0, -29)
	label := "30 дней"
	switch value {
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		label = "Сегодня"
	case "7d":
		start = now.AddDate(0, 0, -6)
		label = "7 дней"
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		label = "Этот месяц"
	}
	return start, end, label
}

func (a *api) dashboardV2(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	start, end, label := dashboardRange(r.URL.Query().Get("period"), now)
	duration := end.Sub(start)
	previousStart, previousEnd := start.Add(-duration), start
	branch := strings.TrimSpace(r.URL.Query().Get("branch"))
	tenant := companyID(r)

	type metrics struct{ visits, customers, returning, rewards int }
	read := func(from, to time.Time) (metrics, error) {
		var result metrics
		err := a.db.QueryRow(r.Context(), `SELECT
			(SELECT count(*) FROM visits v WHERE v.company_id=$1 AND v.created_at >= $2 AND v.created_at < $3 AND v.reversed_at IS NULL AND ($4='' OR v.branch_id=nullif($4,'')::uuid)),
			(SELECT count(*) FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND c.created_at >= $2 AND c.created_at < $3),
			(SELECT count(DISTINCT v.customer_id) FROM visits v WHERE v.company_id=$1 AND v.created_at >= $2 AND v.created_at < $3 AND v.reversed_at IS NULL AND ($4='' OR v.branch_id=nullif($4,'')::uuid) AND EXISTS(SELECT 1 FROM visits old WHERE old.company_id=v.company_id AND old.customer_id=v.customer_id AND old.created_at < $2 AND old.reversed_at IS NULL)),
			(SELECT count(*) FROM customer_rewards cr WHERE cr.company_id=$1 AND cr.issued_at >= $2 AND cr.issued_at < $3 AND cr.status IN('available','reserved','redeemed'))`, tenant, from, to, branch).Scan(&result.visits, &result.customers, &result.returning, &result.rewards)
		return result, err
	}
	current, err := read(start, end)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить обзор")
		return
	}
	previous, err := read(previousStart, previousEnd)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить сравнение")
		return
	}

	var totalCustomers, repeatCustomers, redeemed, issued, closeToReward int
	var averageVisits float64
	err = a.db.QueryRow(r.Context(), `SELECT
		(SELECT count(*) FROM customers WHERE company_id=$1 AND deleted_at IS NULL),
		(SELECT count(*) FROM customers WHERE company_id=$1 AND deleted_at IS NULL AND total_visits>=2),
		(SELECT count(*) FROM customer_rewards WHERE company_id=$1 AND status='redeemed'),
		(SELECT count(*) FROM customer_rewards WHERE company_id=$1 AND status IN('available','reserved','redeemed')),
		(SELECT count(*) FROM customers c CROSS JOIN LATERAL (SELECT coalesce((actions->>'visits')::int,5) target FROM loyalty_rules WHERE company_id=c.company_id AND event_type='visit_milestone' AND is_active ORDER BY priority LIMIT 1) rule WHERE c.company_id=$1 AND c.deleted_at IS NULL AND rule.target>1 AND c.total_visits % rule.target=rule.target-1),
		(SELECT coalesce(avg(total_visits),0) FROM customers WHERE company_id=$1 AND deleted_at IS NULL)`, tenant).Scan(&totalCustomers, &repeatCustomers, &redeemed, &issued, &closeToReward, &averageVisits)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать лояльность")
		return
	}

	trend := []map[string]any{}
	rows, err := a.db.Query(r.Context(), `SELECT day::date,count(v.id) FROM generate_series($2::date,$3::date,interval '1 day') day LEFT JOIN visits v ON v.company_id=$1 AND v.created_at::date=day::date AND v.reversed_at IS NULL AND ($4='' OR v.branch_id=nullif($4,'')::uuid) GROUP BY day ORDER BY day`, tenant, start, end, branch)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var day time.Time
			var count int
			if rows.Scan(&day, &count) == nil {
				trend = append(trend, map[string]any{"date": day, "value": count})
			}
		}
	}
	var branches, employees, devices int
	var programConfigured, rewardConfigured, testCustomer bool
	_ = a.db.QueryRow(r.Context(), `SELECT (SELECT count(*) FROM branches WHERE company_id=$1 AND deleted_at IS NULL),(SELECT count(*) FROM users WHERE company_id=$1 AND role IN('company_owner','employee') AND deleted_at IS NULL),(SELECT count(*) FROM devices WHERE company_id=$1),EXISTS(SELECT 1 FROM company_settings WHERE company_id=$1 AND loyalty_reward_rule_id IS NOT NULL),EXISTS(SELECT 1 FROM reward_definitions WHERE company_id=$1 AND is_active AND deleted_at IS NULL),EXISTS(SELECT 1 FROM visits WHERE company_id=$1 AND reversed_at IS NULL)`, tenant).Scan(&branches, &employees, &devices, &programConfigured, &rewardConfigured, &testCustomer)
	launched := programConfigured && rewardConfigured && devices > 0 && testCustomer
	latestCustomers := []map[string]any{}
	customerRows, customerErr := a.db.Query(r.Context(), `SELECT id,first_name,last_name,phone,total_points,created_at FROM customers WHERE company_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 5`, tenant)
	if customerErr == nil {
		defer customerRows.Close()
		for customerRows.Next() {
			var id, first, last, phone string
			var points int
			var created time.Time
			if customerRows.Scan(&id, &first, &last, &phone, &points, &created) == nil {
				latestCustomers = append(latestCustomers, map[string]any{"id": id, "name": strings.TrimSpace(first + " " + last), "phone": phone, "points": points, "createdAt": created})
			}
		}
	}
	latestVisits := []map[string]any{}
	visitRows, visitErr := a.db.Query(r.Context(), `SELECT v.id,c.first_name,c.last_name,b.name,v.points_added,v.created_at FROM visits v JOIN customers c ON c.id=v.customer_id JOIN branches b ON b.id=v.branch_id WHERE v.company_id=$1 ORDER BY v.created_at DESC LIMIT 5`, tenant)
	if visitErr == nil {
		defer visitRows.Close()
		for visitRows.Next() {
			var id, first, last, branchName string
			var points int
			var created time.Time
			if visitRows.Scan(&id, &first, &last, &branchName, &points, &created) == nil {
				latestVisits = append(latestVisits, map[string]any{"id": id, "customer": strings.TrimSpace(first + " " + last), "branch": branchName, "points": points, "createdAt": created})
			}
		}
	}
	var bonusIssued, bonusRedeemed, scans int
	_ = a.db.QueryRow(r.Context(), `SELECT
		(SELECT coalesce(sum(points_added),0) FROM visits WHERE company_id=$1 AND created_at::date=current_date AND reversed_at IS NULL),
		(SELECT coalesce(sum(amount),0) FROM bonus_ledger WHERE company_id=$1 AND operation='debit' AND created_at::date=current_date),
		(SELECT coalesce(sum(scans_count),0) FROM devices WHERE company_id=$1)`, tenant).Scan(&bonusIssued, &bonusRedeemed, &scans)
	conversion := 0.0
	if scans > 0 {
		conversion = float64(totalCustomers) / float64(scans) * 100
	}

	repeatRate, redemptionRate := 0.0, 0.0
	if totalCustomers > 0 {
		repeatRate = float64(repeatCustomers) * 100 / float64(totalCustomers)
	}
	if issued > 0 {
		redemptionRate = float64(redeemed) * 100 / float64(issued)
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{
		"period":   map[string]any{"label": label, "from": start, "to": end},
		"metrics":  map[string]any{"visits": current.visits, "customers": current.customers, "returning": current.returning, "rewards": current.rewards},
		"previous": map[string]any{"visits": previous.visits, "customers": previous.customers, "returning": previous.returning, "rewards": previous.rewards},
		"loyalty":  map[string]any{"repeatRate": repeatRate, "redemptionRate": redemptionRate, "closeToReward": closeToReward, "averageVisits": averageVisits},
		"trend":    trend, "customers": totalCustomers, "visitsToday": current.visits, "repeatCustomers": repeatCustomers, "rewardsIssued": issued,
		"bonusIssued": bonusIssued, "bonusRedeemed": bonusRedeemed, "nfcConversion": conversion, "latestCustomers": latestCustomers, "latestVisits": latestVisits,
		"onboarding": map[string]bool{"program": programConfigured, "reward": rewardConfigured, "device": devices > 0, "testCustomer": testCustomer, "launched": launched},
	}})
}
