package httpapi

import (
	"net/http"
	"strconv"
)

func (a *api) businessOutcomes(w http.ResponseWriter, r *http.Request) {
	days := 30
	if value, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && value >= 7 && value <= 365 {
		days = value
	}
	tenant := companyID(r)
	var returnedCustomers, repeatVisits int
	err := a.db.QueryRow(r.Context(), `SELECT count(DISTINCT customer_id) FROM customer_events WHERE company_id=$1 AND event_type='customer.returned' AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2)`, tenant, days).Scan(&returnedCustomers)
	if err == nil {
		err = a.db.QueryRow(r.Context(), `WITH ranked AS (SELECT customer_id,occurred_at,row_number() OVER(PARTITION BY customer_id ORDER BY occurred_at,id) n FROM customer_events WHERE company_id=$1 AND event_type='visit.completed' AND NOT sandbox) SELECT count(*) FROM ranked WHERE n>=2 AND occurred_at>=now()-make_interval(days=>$2)`, tenant, days).Scan(&repeatVisits)
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать возвращаемость")
		return
	}

	var automationMessages, automationCustomers, automationReturned int
	var automationRevenue float64
	err = a.db.QueryRow(r.Context(), `WITH sent AS (
		SELECT customer_id,min(occurred_at) sent_at,count(*) messages FROM customer_events WHERE company_id=$1 AND event_type='campaign.sent' AND source='automation' AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2) GROUP BY customer_id
	), purchases AS (
		SELECT s.customer_id,sum(t.net_amount) revenue FROM sent s JOIN sales_transactions t ON t.company_id=$1 AND t.customer_id=s.customer_id AND t.status='completed' AND NOT t.sandbox AND t.occurred_at>=s.sent_at AND t.occurred_at<=s.sent_at+interval '30 days' GROUP BY s.customer_id
	) SELECT coalesce(sum(s.messages),0),count(s.customer_id),count(p.customer_id),coalesce(sum(p.revenue),0) FROM sent s LEFT JOIN purchases p ON p.customer_id=s.customer_id`, tenant, days).Scan(&automationMessages, &automationCustomers, &automationReturned, &automationRevenue)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать результат автоматизаций")
		return
	}

	var referralCustomers, referralRepeat int
	var referralRevenue float64
	err = a.db.QueryRow(r.Context(), `WITH converted AS (
		SELECT DISTINCT customer_id FROM customer_events WHERE company_id=$1 AND event_type='referral.converted' AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2)
	), purchases AS (
		SELECT c.customer_id,count(t.id) purchase_count,coalesce(sum(t.net_amount),0) revenue FROM converted c LEFT JOIN sales_transactions t ON t.company_id=$1 AND t.customer_id=c.customer_id AND t.status='completed' AND NOT t.sandbox GROUP BY c.customer_id
	) SELECT count(*),count(*) FILTER(WHERE purchase_count>=2),coalesce(sum(revenue),0) FROM purchases`, tenant, days).Scan(&referralCustomers, &referralRepeat, &referralRevenue)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать результат рекомендаций")
		return
	}

	var rewardName string
	var rewardRedemptions int
	_ = a.db.QueryRow(r.Context(), `SELECT d.name,count(*) FROM customer_rewards cr JOIN reward_definitions d ON d.id=cr.definition_id AND d.company_id=cr.company_id WHERE cr.company_id=$1 AND cr.status='redeemed' AND cr.redeemed_at>=now()-make_interval(days=>$2) GROUP BY d.id,d.name ORDER BY count(*) DESC,d.name LIMIT 1`, tenant, days).Scan(&rewardName, &rewardRedemptions)
	var memberRevenue, attributedCampaignRevenue float64
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce(sum(net_amount),0) FROM sales_transactions WHERE company_id=$1 AND customer_id IS NOT NULL AND status='completed' AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2)`, tenant, days).Scan(&memberRevenue)
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce(sum(conversion_value),0) FROM campaign_conversions WHERE company_id=$1 AND conversion_type='purchased' AND occurred_at>=now()-make_interval(days=>$2)`, tenant, days).Scan(&attributedCampaignRevenue)

	write(w, 200, envelope{Success: true, Data: map[string]any{
		"days":        days,
		"retention":   map[string]any{"returnedCustomers": returnedCustomers, "repeatVisits": repeatVisits},
		"automations": map[string]any{"messages": automationMessages, "reachedCustomers": automationCustomers, "returnedCustomers": automationReturned, "attributedRevenue": automationRevenue},
		"referrals":   map[string]any{"newCustomers": referralCustomers, "repeatCustomers": referralRepeat, "revenue": referralRevenue},
		"rewards":     map[string]any{"bestName": rewardName, "redemptions": rewardRedemptions},
		"revenue":     map[string]any{"members": memberRevenue, "campaignAttributed": attributedCampaignRevenue},
	}})
}
