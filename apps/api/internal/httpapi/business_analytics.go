package httpapi

import (
	"net/http"
)

type repeatPurchaseMetric struct {
	Days               int     `json:"days"`
	Customers          int     `json:"customers"`
	RepeatCustomers    int     `json:"repeatCustomers"`
	RepeatPurchaseRate float64 `json:"repeatPurchaseRate"`
}

type averageCheckBreakdown struct {
	Overall      float64 `json:"overall"`
	Participants float64 `json:"participants"`
	Anonymous    float64 `json:"anonymous"`
	NewCustomers float64 `json:"newCustomers"`
	Repeat       float64 `json:"repeatCustomers"`
}

func percentage(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

func (a *api) businessAnalytics(w http.ResponseWriter, r *http.Request) {
	tenant := companyID(r)

	repeatWindows := make([]repeatPurchaseMetric, 0, 3)
	for _, days := range []int{30, 60, 90} {
		var customers, repeatCustomers int
		err := a.db.QueryRow(r.Context(), `WITH purchases AS (
			SELECT customer_id,count(*) AS purchase_count
			FROM sales_transactions
			WHERE company_id=$1 AND status='completed' AND customer_id IS NOT NULL
			  AND occurred_at>=now()-make_interval(days=>$2)
			GROUP BY customer_id
		) SELECT count(*),count(*) FILTER(WHERE purchase_count>=2) FROM purchases`, tenant, days).Scan(&customers, &repeatCustomers)
		if err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать повторные покупки")
			return
		}
		repeatWindows = append(repeatWindows, repeatPurchaseMetric{
			Days: days, Customers: customers, RepeatCustomers: repeatCustomers,
			RepeatPurchaseRate: percentage(repeatCustomers, customers),
		})
	}

	var buyers, secondBuyers int
	var averageDaysToSecond float64
	err := a.db.QueryRow(r.Context(), `WITH ranked AS (
		SELECT customer_id,occurred_at,row_number() OVER(PARTITION BY customer_id ORDER BY occurred_at,id) AS purchase_number
		FROM sales_transactions
		WHERE company_id=$1 AND status='completed' AND customer_id IS NOT NULL
	), customer_purchases AS (
		SELECT customer_id,
			min(occurred_at) FILTER(WHERE purchase_number=1) AS first_purchase,
			min(occurred_at) FILTER(WHERE purchase_number=2) AS second_purchase
		FROM ranked GROUP BY customer_id
	) SELECT count(*),count(*) FILTER(WHERE second_purchase IS NOT NULL),
		coalesce(avg(extract(epoch FROM(second_purchase-first_purchase))/86400.0) FILTER(WHERE second_purchase IS NOT NULL),0)
	FROM customer_purchases`, tenant).Scan(&buyers, &secondBuyers, &averageDaysToSecond)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать путь до второй покупки")
		return
	}

	var checks averageCheckBreakdown
	err = a.db.QueryRow(r.Context(), `WITH completed AS (
		SELECT net_amount,customer_id,
			row_number() OVER(PARTITION BY customer_id ORDER BY occurred_at,id) AS customer_purchase_number
		FROM sales_transactions WHERE company_id=$1 AND status='completed'
	) SELECT
		coalesce(avg(net_amount),0),
		coalesce(avg(net_amount) FILTER(WHERE customer_id IS NOT NULL),0),
		coalesce(avg(net_amount) FILTER(WHERE customer_id IS NULL),0),
		coalesce(avg(net_amount) FILTER(WHERE customer_id IS NOT NULL AND customer_purchase_number=1),0),
		coalesce(avg(net_amount) FILTER(WHERE customer_id IS NOT NULL AND customer_purchase_number>=2),0)
	FROM completed`, tenant).Scan(&checks.Overall, &checks.Participants, &checks.Anonymous, &checks.NewCustomers, &checks.Repeat)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать средний чек")
		return
	}

	branches := []map[string]any{}
	branchRows, err := a.db.Query(r.Context(), `SELECT coalesce(b.id::text,''),coalesce(b.name,'Без филиала'),
		count(t.id),coalesce(sum(t.net_amount),0),coalesce(avg(t.net_amount),0),
		count(DISTINCT t.customer_id) FILTER(WHERE t.customer_id IS NOT NULL)
	FROM sales_transactions t LEFT JOIN branches b ON b.id=t.branch_id
	WHERE t.company_id=$1 AND t.status='completed'
	GROUP BY b.id,b.name ORDER BY sum(t.net_amount) DESC NULLS LAST`, tenant)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать показатели филиалов")
		return
	}
	defer branchRows.Close()
	for branchRows.Next() {
		var id, name string
		var transactions, customers int
		var revenue, averageCheck float64
		if err = branchRows.Scan(&id, &name, &transactions, &revenue, &averageCheck, &customers); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось прочитать показатели филиалов")
			return
		}
		branches = append(branches, map[string]any{"id": id, "name": name, "transactions": transactions, "customers": customers, "revenue": revenue, "averageCheck": averageCheck})
	}

	var totalRevenue, averageLTV, medianLTV, maximumLTV float64
	var customersWithPurchases int
	err = a.db.QueryRow(r.Context(), `WITH customer_ltv AS (
		SELECT customer_id,sum(net_amount)::numeric AS ltv
		FROM sales_transactions
		WHERE company_id=$1 AND status='completed' AND customer_id IS NOT NULL
		GROUP BY customer_id
	) SELECT coalesce(sum(ltv),0),coalesce(avg(ltv),0),
		coalesce(percentile_cont(0.5) WITHIN GROUP(ORDER BY ltv),0),coalesce(max(ltv),0),count(*)
	FROM customer_ltv`, tenant).Scan(&totalRevenue, &averageLTV, &medianLTV, &maximumLTV, &customersWithPurchases)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать LTV")
		return
	}

	rfmSegments := []map[string]any{}
	rfmRows, err := a.db.Query(r.Context(), `WITH completed AS (
		SELECT customer_id,occurred_at,net_amount,
			lag(occurred_at) OVER(PARTITION BY customer_id ORDER BY occurred_at,id) AS previous_purchase
		FROM sales_transactions
		WHERE company_id=$1 AND status='completed' AND customer_id IS NOT NULL
	), per_customer AS (
		SELECT customer_id,max(occurred_at) AS last_purchase,count(*) AS frequency,
			sum(net_amount) AS monetary,
			percentile_cont(0.5) WITHIN GROUP(ORDER BY extract(epoch FROM(occurred_at-previous_purchase))/86400.0)
				FILTER(WHERE previous_purchase IS NOT NULL) AS expected_interval
		FROM completed GROUP BY customer_id
	), company_interval AS (
		SELECT coalesce(percentile_cont(0.5) WITHIN GROUP(ORDER BY expected_interval) FILTER(WHERE expected_interval IS NOT NULL),30) AS fallback
		FROM per_customer
	), scored AS (
		SELECT p.*,
			extract(epoch FROM(now()-last_purchase))/86400.0 AS days_since,
			ntile(5) OVER(ORDER BY last_purchase) AS r,
			ntile(5) OVER(ORDER BY frequency) AS f,
			ntile(5) OVER(ORDER BY monetary) AS m,
			coalesce(expected_interval,company_interval.fallback) AS expected
		FROM per_customer p CROSS JOIN company_interval
	), classified AS (
		SELECT *,CASE
			WHEN r>=4 AND f>=4 AND m>=4 THEN 'champions'
			WHEN r>=3 AND f>=4 THEN 'loyal'
			WHEN r>=4 AND f BETWEEN 2 AND 3 THEN 'potential_loyalist'
			WHEN r=5 AND frequency=1 THEN 'new'
			WHEN r BETWEEN 2 AND 3 AND f>=2 THEN 'need_attention'
			WHEN r<=2 AND f>=3 THEN 'at_risk'
			ELSE 'lost' END AS segment,
			CASE WHEN days_since<1.2*expected THEN 'low' WHEN days_since<=2.0*expected THEN 'medium' ELSE 'high' END AS risk
		FROM scored
	) SELECT segment,risk,count(*),coalesce(sum(monetary),0),coalesce(avg(monetary),0)
	FROM classified GROUP BY segment,risk ORDER BY sum(monetary) DESC`, tenant)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать RFM")
		return
	}
	defer rfmRows.Close()
	segmentNames := map[string]string{
		"champions": "Лучшие клиенты", "loyal": "Постоянные", "potential_loyalist": "Потенциально постоянные",
		"new": "Новые", "need_attention": "Требуют внимания", "at_risk": "Под угрозой", "lost": "Потерянные",
	}
	for rfmRows.Next() {
		var segment, risk string
		var customers int
		var revenue, averageLTV float64
		if err = rfmRows.Scan(&segment, &risk, &customers, &revenue, &averageLTV); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось прочитать RFM")
			return
		}
		rfmSegments = append(rfmSegments, map[string]any{"code": segment, "name": segmentNames[segment], "churnRisk": risk, "customers": customers, "revenue": revenue, "averageLTV": averageLTV})
	}

	funnelTypes := []string{"smart_link_opened", "registration_started", "customer_registered", "first_purchase_completed", "second_purchase_completed", "reward_earned", "reward_redeemed"}
	funnelCounts := map[string]int{}
	funnelRows, err := a.db.Query(r.Context(), `SELECT event_type,count(*) FROM customer_events
		WHERE company_id=$1 AND event_type=ANY($2::varchar[]) GROUP BY event_type`, tenant, funnelTypes)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать воронку")
		return
	}
	defer funnelRows.Close()
	for funnelRows.Next() {
		var eventType string
		var count int
		if err = funnelRows.Scan(&eventType, &count); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось прочитать воронку")
			return
		}
		funnelCounts[eventType] = count
	}
	funnel := make([]map[string]any, 0, len(funnelTypes))
	previous := 0
	for index, eventType := range funnelTypes {
		count := funnelCounts[eventType]
		conversion := 0.0
		if index == 0 {
			if count > 0 {
				conversion = 100
			}
		} else {
			conversion = percentage(count, previous)
		}
		funnel = append(funnel, map[string]any{"eventType": eventType, "count": count, "conversionFromPrevious": conversion})
		previous = count
	}

	write(w, 200, envelope{Success: true, Data: map[string]any{
		"currency": "KZT",
		"repeatPurchase": map[string]any{
			"windows":                     repeatWindows,
			"averageDaysToSecondPurchase": averageDaysToSecond,
			"customersWithPurchase":       buyers,
			"customersWithSecondPurchase": secondBuyers,
			"secondPurchaseConversion":    percentage(secondBuyers, buyers),
		},
		"averageCheck": checks,
		"ltv": map[string]any{
			"type": "historical", "customers": customersWithPurchases, "totalRevenue": totalRevenue,
			"average": averageLTV, "median": medianLTV, "maximum": maximumLTV,
			"predictedAvailable": false,
		},
		"rfm":      map[string]any{"windowDays": 180, "segments": rfmSegments},
		"branches": branches,
		"funnel":   funnel,
		"limitations": []string{
			"predicted_ltv_requires_6_12_months_of_data",
			"campaign_uplift_requires_holdout_group",
			"bonus_liability_requires_expiring_bonus_lots",
		},
	}})
}
