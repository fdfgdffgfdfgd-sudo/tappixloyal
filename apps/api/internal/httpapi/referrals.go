package httpapi

import (
	"net/http"
	"strings"
	"time"
)

type referralProgramInput struct {
	Name                  string  `json:"name"`
	Status                string  `json:"status"`
	ReferrerRewardValue   float64 `json:"referrerRewardValue"`
	FriendRewardValue     float64 `json:"friendRewardValue"`
	MinimumPurchaseAmount float64 `json:"minimumPurchaseAmount"`
	RewardDelayDays       int     `json:"rewardDelayDays"`
	MaxRewardsPerCustomer int     `json:"maxRewardsPerCustomer"`
	MaxRewardsPerMonth    int     `json:"maxRewardsPerMonth"`
}

func (a *api) getReferralProgram(w http.ResponseWriter, r *http.Request) {
	var id, name, status string
	var referrer, friend, minimum float64
	var delay int
	var perCustomer, perMonth *int
	var updated time.Time
	err := a.db.QueryRow(r.Context(), `SELECT id,name,status,referrer_reward_value,friend_reward_value,minimum_purchase_amount,reward_delay_days,max_rewards_per_customer,max_rewards_per_month,updated_at
		FROM referral_programs WHERE company_id=$1 AND status<>'archived' ORDER BY created_at DESC LIMIT 1`, companyID(r)).Scan(&id, &name, &status, &referrer, &friend, &minimum, &delay, &perCustomer, &perMonth, &updated)
	if err != nil {
		write(w, 200, envelope{Success: true, Data: nil})
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": id, "name": name, "status": status, "referrerRewardValue": referrer, "friendRewardValue": friend, "minimumPurchaseAmount": minimum, "rewardDelayDays": delay, "maxRewardsPerCustomer": perCustomer, "maxRewardsPerMonth": perMonth, "updatedAt": updated}})
}

func (a *api) saveReferralProgram(w http.ResponseWriter, r *http.Request) {
	var in referralProgramInput
	if !decode(w, r, &in) {
		return
	}
	in.Name, in.Status = strings.TrimSpace(in.Name), strings.ToLower(strings.TrimSpace(in.Status))
	if in.Name == "" || (in.Status != "draft" && in.Status != "active" && in.Status != "paused") || in.ReferrerRewardValue < 0 || in.FriendRewardValue < 0 || in.MinimumPurchaseAmount < 0 || in.RewardDelayDays < 0 || in.RewardDelayDays > 365 || in.MaxRewardsPerCustomer < 0 || in.MaxRewardsPerMonth < 0 {
		fail(w, 422, "INVALID_REFERRAL_PROGRAM", "Проверьте награды, лимиты и условия программы")
		return
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var id string
	err := a.db.QueryRow(r.Context(), `WITH current AS (SELECT id FROM referral_programs WHERE company_id=$1 AND status<>'archived' ORDER BY created_at DESC LIMIT 1)
		INSERT INTO referral_programs(id,company_id,name,status,referrer_reward_type,referrer_reward_value,friend_reward_type,friend_reward_value,qualification_event,minimum_purchase_amount,reward_delay_days,max_rewards_per_customer,max_rewards_per_month,created_by)
		VALUES(coalesce((SELECT id FROM current),gen_random_uuid()),$1,$2,$3,'points',$4,'points',$5,'first_paid_purchase',$6,$7,nullif($8,0),nullif($9,0),$10)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name,status=excluded.status,referrer_reward_value=excluded.referrer_reward_value,friend_reward_value=excluded.friend_reward_value,minimum_purchase_amount=excluded.minimum_purchase_amount,reward_delay_days=excluded.reward_delay_days,max_rewards_per_customer=excluded.max_rewards_per_customer,max_rewards_per_month=excluded.max_rewards_per_month,updated_at=now() RETURNING id`, companyID(r), in.Name, in.Status, in.ReferrerRewardValue, in.FriendRewardValue, in.MinimumPurchaseAmount, in.RewardDelayDays, in.MaxRewardsPerCustomer, in.MaxRewardsPerMonth, claims.Subject).Scan(&id)
	if err != nil {
		fail(w, 500, "REFERRAL_PROGRAM_SAVE_FAILED", "Не удалось сохранить реферальную программу")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": id, "status": in.Status}})
}

func (a *api) referralAnalytics(w http.ResponseWriter, r *http.Request) {
	var clicks, registrations, qualified, rewarded int
	var revenue, rewards float64
	_ = a.db.QueryRow(r.Context(), `SELECT count(*),count(*) FILTER(WHERE status IN('registered','qualified','reward_pending','rewarded')),count(*) FILTER(WHERE status IN('qualified','reward_pending','rewarded')),count(*) FILTER(WHERE status='rewarded'),coalesce(sum(t.net_amount),0)
		FROM referral_attributions a LEFT JOIN sales_transactions t ON t.id=a.qualifying_transaction_id WHERE a.company_id=$1`, companyID(r)).Scan(&clicks, &registrations, &qualified, &rewarded, &revenue)
	_ = a.db.QueryRow(r.Context(), `SELECT coalesce(sum(reward_value),0) FROM referral_rewards WHERE company_id=$1 AND status='issued'`, companyID(r)).Scan(&rewards)
	rows, err := a.db.Query(r.Context(), `SELECT c.id,trim(c.first_name||' '||c.last_name),c.referral_code,count(a.id) FILTER(WHERE a.status IN('registered','qualified','reward_pending','rewarded')),count(a.id) FILTER(WHERE a.status='rewarded'),coalesce(sum(t.net_amount),0)
		FROM customers c JOIN referral_attributions a ON a.referrer_customer_id=c.id AND a.company_id=c.company_id LEFT JOIN sales_transactions t ON t.id=a.qualifying_transaction_id
		WHERE c.company_id=$1 GROUP BY c.id ORDER BY count(a.id) FILTER(WHERE a.status IN('registered','qualified','reward_pending','rewarded')) DESC LIMIT 10`, companyID(r))
	leaders := []map[string]any{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, name, code string
			var invited, paid int
			var amount float64
			if rows.Scan(&id, &name, &code, &invited, &paid, &amount) == nil {
				leaders = append(leaders, map[string]any{"customerId": id, "name": name, "referralCode": code, "invited": invited, "rewarded": paid, "revenue": amount})
			}
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"clicks": clicks, "registrations": registrations, "qualified": qualified, "rewarded": rewarded, "revenue": revenue, "rewardCost": rewards, "conversionRate": percentage(registrations, clicks), "qualificationRate": percentage(qualified, registrations), "leaders": leaders}})
}
