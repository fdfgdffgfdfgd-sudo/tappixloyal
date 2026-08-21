package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

func (a *api) businessOutcomes(w http.ResponseWriter, r *http.Request) {
	days := normalizedOutcomeDays(r.URL.Query().Get("days"))
	branch := strings.TrimSpace(r.URL.Query().Get("branchId"))
	if len(branch) != 36 || strings.Count(branch, "-") != 4 {
		branch = ""
	}
	tenant := companyID(r)
	var returnedCustomers, repeatVisits int
	err := a.db.QueryRow(r.Context(), `SELECT count(DISTINCT customer_id) FROM customer_events WHERE company_id=$1 AND event_type='customer.returned' AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2) AND ($3='' OR branch_id=nullif($3,'')::uuid)`, tenant, days, branch).Scan(&returnedCustomers)
	if err == nil {
		err = a.db.QueryRow(r.Context(), `WITH ranked AS (SELECT customer_id,occurred_at,row_number() OVER(PARTITION BY customer_id ORDER BY occurred_at,id) n FROM customer_events WHERE company_id=$1 AND event_type='visit.completed' AND NOT sandbox AND ($3='' OR branch_id=nullif($3,'')::uuid)) SELECT count(*) FROM ranked WHERE n>=2 AND occurred_at>=now()-make_interval(days=>$2)`, tenant, days, branch).Scan(&repeatVisits)
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать возвращаемость")
		return
	}

	var automationMessages, automationCustomers, automationReturned int
	var automationRevenue float64
	err = a.db.QueryRow(r.Context(), `WITH sent AS (
		SELECT customer_id,min(occurred_at) sent_at,count(*) messages FROM customer_events WHERE company_id=$1 AND event_type='campaign.sent' AND source='automation' AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2) AND ($3='' OR branch_id=nullif($3,'')::uuid) GROUP BY customer_id
	), purchases AS (
		SELECT s.customer_id,sum(t.net_amount) revenue FROM sent s JOIN sales_transactions t ON t.company_id=$1 AND t.customer_id=s.customer_id AND t.status='completed' AND NOT t.sandbox AND t.occurred_at>=s.sent_at AND t.occurred_at<=s.sent_at+interval '30 days' AND ($3='' OR t.branch_id=nullif($3,'')::uuid) GROUP BY s.customer_id
	) SELECT coalesce(sum(s.messages),0),count(s.customer_id),count(p.customer_id),coalesce(sum(p.revenue),0) FROM sent s LEFT JOIN purchases p ON p.customer_id=s.customer_id`, tenant, days, branch).Scan(&automationMessages, &automationCustomers, &automationReturned, &automationRevenue)
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
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce(sum(net_amount),0) FROM sales_transactions WHERE company_id=$1 AND customer_id IS NOT NULL AND status='completed' AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2) AND ($3='' OR branch_id=nullif($3,'')::uuid)`, tenant, days, branch).Scan(&memberRevenue)
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce(sum(conversion_value),0) FROM campaign_conversions WHERE company_id=$1 AND conversion_type='purchased' AND occurred_at>=now()-make_interval(days=>$2)`, tenant, days).Scan(&attributedCampaignRevenue)

	var previousReturned, previousAutomationReturned, previousReferralCustomers, previousRewardRedemptions int
	var previousAutomationRevenue, previousReferralRevenue, previousMemberRevenue float64
	err = a.db.QueryRow(r.Context(), `WITH bounds AS (
		SELECT now()-make_interval(days=>$2*2) period_start,now()-make_interval(days=>$2) period_end
	), automation_sent AS (
		SELECT customer_id,min(occurred_at) sent_at FROM customer_events,bounds WHERE company_id=$1 AND event_type='campaign.sent' AND source='automation' AND NOT sandbox AND occurred_at>=period_start AND occurred_at<period_end GROUP BY customer_id
	), automation_purchases AS (
		SELECT s.customer_id,sum(t.net_amount) revenue FROM automation_sent s JOIN sales_transactions t ON t.company_id=$1 AND t.customer_id=s.customer_id AND t.status='completed' AND NOT t.sandbox AND t.occurred_at>=s.sent_at AND t.occurred_at<=s.sent_at+interval '30 days' GROUP BY s.customer_id
	), referred AS (
		SELECT DISTINCT customer_id FROM customer_events,bounds WHERE company_id=$1 AND event_type='referral.converted' AND NOT sandbox AND occurred_at>=period_start AND occurred_at<period_end
	), referral_revenue AS (
		SELECT coalesce(sum(t.net_amount),0) revenue FROM referred r JOIN sales_transactions t ON t.company_id=$1 AND t.customer_id=r.customer_id AND t.status='completed' AND NOT t.sandbox
	) SELECT
		(SELECT count(DISTINCT customer_id) FROM customer_events,bounds WHERE company_id=$1 AND event_type='customer.returned' AND NOT sandbox AND occurred_at>=period_start AND occurred_at<period_end),
		(SELECT count(*) FROM automation_purchases),
		(SELECT coalesce(sum(revenue),0) FROM automation_purchases),
		(SELECT count(*) FROM referred),
		(SELECT revenue FROM referral_revenue),
		(SELECT count(*) FROM customer_rewards cr,bounds WHERE cr.company_id=$1 AND cr.status='redeemed' AND cr.redeemed_at>=period_start AND cr.redeemed_at<period_end),
		(SELECT coalesce(sum(net_amount),0) FROM sales_transactions,bounds WHERE company_id=$1 AND customer_id IS NOT NULL AND status='completed' AND NOT sandbox AND occurred_at>=period_start AND occurred_at<period_end)`, tenant, days).Scan(&previousReturned, &previousAutomationReturned, &previousAutomationRevenue, &previousReferralCustomers, &previousReferralRevenue, &previousRewardRedemptions, &previousMemberRevenue)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сравнить периоды")
		return
	}

	type branchOutcome struct {
		ID                string  `json:"id"`
		Name              string  `json:"name"`
		Customers         int     `json:"customers"`
		ReturnedCustomers int     `json:"returnedCustomers"`
		Visits            int     `json:"visits"`
		Revenue           float64 `json:"revenue"`
		Rewards           int     `json:"rewards"`
	}
	branches := []branchOutcome{}
	rows, branchErr := a.db.Query(r.Context(), `WITH event_totals AS (
		SELECT branch_id,count(DISTINCT customer_id) customers,count(*) FILTER(WHERE event_type='visit.completed') visits,count(DISTINCT customer_id) FILTER(WHERE event_type='customer.returned') returned
		FROM customer_events WHERE company_id=$1 AND branch_id IS NOT NULL AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2) GROUP BY branch_id
	), transaction_totals AS (
		SELECT branch_id,coalesce(sum(net_amount),0) revenue FROM sales_transactions WHERE company_id=$1 AND branch_id IS NOT NULL AND status='completed' AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2) GROUP BY branch_id
	), reward_totals AS (
		SELECT branch_id,count(*) rewards FROM customer_events WHERE company_id=$1 AND event_type='reward.redeemed' AND branch_id IS NOT NULL AND NOT sandbox AND occurred_at>=now()-make_interval(days=>$2) GROUP BY branch_id
	) SELECT b.id,b.name,coalesce(e.customers,0),coalesce(e.returned,0),coalesce(e.visits,0),coalesce(t.revenue,0),coalesce(r.rewards,0)
	FROM branches b LEFT JOIN event_totals e ON e.branch_id=b.id LEFT JOIN transaction_totals t ON t.branch_id=b.id LEFT JOIN reward_totals r ON r.branch_id=b.id
	WHERE b.company_id=$1 AND b.deleted_at IS NULL ORDER BY coalesce(t.revenue,0) DESC,coalesce(e.returned,0) DESC,b.name`, tenant, days)
	if branchErr == nil {
		defer rows.Close()
		for rows.Next() {
			var item branchOutcome
			if scanErr := rows.Scan(&item.ID, &item.Name, &item.Customers, &item.ReturnedCustomers, &item.Visits, &item.Revenue, &item.Rewards); scanErr != nil {
				branchErr = scanErr
				break
			}
			branches = append(branches, item)
		}
		if branchErr == nil {
			branchErr = rows.Err()
		}
	}
	if branchErr != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось рассчитать результаты филиалов")
		return
	}

	write(w, 200, envelope{Success: true, Data: map[string]any{
		"days":        days,
		"retention":   map[string]any{"returnedCustomers": returnedCustomers, "repeatVisits": repeatVisits},
		"automations": map[string]any{"messages": automationMessages, "reachedCustomers": automationCustomers, "returnedCustomers": automationReturned, "attributedRevenue": automationRevenue},
		"referrals":   map[string]any{"newCustomers": referralCustomers, "repeatCustomers": referralRepeat, "revenue": referralRevenue},
		"rewards":     map[string]any{"bestName": rewardName, "redemptions": rewardRedemptions},
		"revenue":     map[string]any{"members": memberRevenue, "campaignAttributed": attributedCampaignRevenue},
		"previous": map[string]any{
			"returnedCustomers":  previousReturned,
			"automationReturned": previousAutomationReturned,
			"automationRevenue":  previousAutomationRevenue,
			"referralCustomers":  previousReferralCustomers,
			"referralRevenue":    previousReferralRevenue,
			"rewardRedemptions":  previousRewardRedemptions,
			"memberRevenue":      previousMemberRevenue,
		},
		"branches": branches,
	}})
}

func normalizedOutcomeDays(raw string) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 7 || value > 365 {
		return 30
	}
	return value
}
