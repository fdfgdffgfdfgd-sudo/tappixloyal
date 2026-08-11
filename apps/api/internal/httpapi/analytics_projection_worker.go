package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func StartAnalyticsProjectionWorker(ctx context.Context, db *pgxpool.Pool) {
	go func() {
		processAnalyticsJobs(ctx, db)
		nextDaily := time.NewTimer(time.Minute)
		jobs := time.NewTicker(15 * time.Second)
		defer nextDaily.Stop()
		defer jobs.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-jobs.C:
				processAnalyticsJobs(ctx, db)
			case <-nextDaily.C:
				expireBonusLots(ctx, db, "")
				refreshAllAnalytics(ctx, db)
				nextDaily.Reset(24 * time.Hour)
			}
		}
	}()
}

func processAnalyticsJobs(ctx context.Context, db *pgxpool.Pool) {
	for i := 0; i < 10; i++ {
		var id, companyID string
		err := db.QueryRow(ctx, `UPDATE integration_jobs SET status='processing',started_at=now(),attempts=attempts+1
			WHERE id=(SELECT id FROM integration_jobs WHERE job_type='analytics_projection' AND status IN('pending','failed') AND available_at<=now() ORDER BY available_at FOR UPDATE SKIP LOCKED LIMIT 1)
			RETURNING id,company_id`).Scan(&id, &companyID)
		if err != nil {
			return
		}
		if err = refreshAnalyticsCompany(ctx, db, companyID); err != nil {
			_, _ = db.Exec(ctx, `UPDATE integration_jobs SET status=CASE WHEN attempts>=max_attempts THEN 'dead' ELSE 'failed' END,last_error=$2,available_at=now()+make_interval(secs=>least(3600,power(2,attempts)::integer)) WHERE id=$1`, id, err.Error())
			slog.Warn("analytics projection failed", "company", companyID, "error", err)
			continue
		}
		_, _ = db.Exec(ctx, `UPDATE integration_jobs SET status='succeeded',completed_at=now(),last_error=NULL WHERE id=$1`, id)
	}
}

func refreshAllAnalytics(ctx context.Context, db *pgxpool.Pool) {
	rows, err := db.Query(ctx, `SELECT id FROM companies WHERE status='active' AND deleted_at IS NULL`)
	if err != nil {
		return
	}
	defer rows.Close()
	companies := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			companies = append(companies, id)
		}
	}
	for _, companyID := range companies {
		if err = refreshAnalyticsCompany(ctx, db, companyID); err != nil {
			slog.Warn("daily analytics projection failed", "company", companyID, "error", err)
		}
	}
}

func refreshAnalyticsCompany(ctx context.Context, db *pgxpool.Pool, companyID string) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "analytics:"+companyID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM analytics_daily_facts WHERE company_id=$1 AND fact_date>=current_date-365`, companyID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `WITH transaction_facts AS (
		SELECT occurred_at::date AS fact_date,branch_id,
			count(*) FILTER(WHERE status IN('completed','partially_refunded','refunded')) AS completed_transactions,
			count(*) FILTER(WHERE status='completed' AND original_transaction_id IS NOT NULL) AS refunded_transactions,
			count(DISTINCT customer_id) FILTER(WHERE customer_id IS NOT NULL) AS active_customers,
			coalesce(sum(gross_amount) FILTER(WHERE original_transaction_id IS NULL),0) AS gross_revenue,
			coalesce(sum(CASE WHEN original_transaction_id IS NULL THEN net_amount ELSE -net_amount END),0) AS net_revenue,
			coalesce(sum(discount_amount) FILTER(WHERE original_transaction_id IS NULL),0) AS discount_amount
		FROM sales_transactions WHERE company_id=$1 AND NOT sandbox AND occurred_at::date>=current_date-365
		GROUP BY occurred_at::date,branch_id
	), customer_first AS (
		SELECT customer_id,min(occurred_at)::date AS first_date FROM sales_transactions
		WHERE company_id=$1 AND status IN('completed','partially_refunded','refunded') AND original_transaction_id IS NULL AND customer_id IS NOT NULL AND NOT sandbox GROUP BY customer_id
	), enriched AS (
		SELECT t.*,
			count(DISTINCT s.customer_id) FILTER(WHERE f.first_date=t.fact_date) AS new_buyers,
			count(DISTINCT s.customer_id) FILTER(WHERE f.first_date<t.fact_date) AS repeat_buyers
		FROM transaction_facts t LEFT JOIN sales_transactions s ON s.company_id=$1 AND s.branch_id IS NOT DISTINCT FROM t.branch_id AND s.occurred_at::date=t.fact_date AND s.customer_id IS NOT NULL AND NOT s.sandbox
		LEFT JOIN customer_first f ON f.customer_id=s.customer_id GROUP BY t.fact_date,t.branch_id,t.completed_transactions,t.refunded_transactions,t.active_customers,t.gross_revenue,t.net_revenue,t.discount_amount
	), registrations AS (
		SELECT created_at::date fact_date,count(*) registrations FROM customers WHERE company_id=$1 AND deleted_at IS NULL AND created_at::date>=current_date-365 GROUP BY created_at::date
	), bonus AS (
		SELECT created_at::date fact_date,
			coalesce(sum(amount) FILTER(WHERE operation='credit'),0) issued,
			coalesce(sum(amount) FILTER(WHERE operation='debit'),0) redeemed,
			coalesce(sum(amount) FILTER(WHERE operation='expire'),0) expired
		FROM bonus_ledger WHERE company_id=$1 AND created_at::date>=current_date-365 GROUP BY created_at::date
	) INSERT INTO analytics_daily_facts(company_id,fact_date,branch_id,registrations,active_customers,new_buyers,repeat_buyers,completed_transactions,refunded_transactions,gross_revenue,net_revenue,discount_amount,bonus_issued_value,bonus_redeemed_value,bonus_expired_value)
	SELECT $1,e.fact_date,e.branch_id,coalesce(r.registrations,0),e.active_customers,e.new_buyers,e.repeat_buyers,e.completed_transactions,e.refunded_transactions,e.gross_revenue,e.net_revenue,e.discount_amount,coalesce(b.issued,0),coalesce(b.redeemed,0),coalesce(b.expired,0)
	FROM enriched e LEFT JOIN registrations r USING(fact_date) LEFT JOIN bonus b USING(fact_date)`, companyID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM analytics_customer_features WHERE company_id=$1`, companyID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `WITH purchases AS (
		SELECT customer_id,occurred_at,net_amount,
			lag(occurred_at) OVER(PARTITION BY customer_id ORDER BY occurred_at,id) previous_purchase
		FROM sales_transactions WHERE company_id=$1 AND customer_id IS NOT NULL AND original_transaction_id IS NULL AND status IN('completed','partially_refunded','refunded') AND NOT sandbox
	), summaries AS (
		SELECT customer_id,min(occurred_at) first_purchase,max(occurred_at) last_purchase,count(*) purchase_count,sum(net_amount) lifetime_revenue,avg(net_amount) average_check,
			percentile_cont(0.5) WITHIN GROUP(ORDER BY extract(epoch FROM(occurred_at-previous_purchase))/86400.0) FILTER(WHERE previous_purchase IS NOT NULL) median_interval
		FROM purchases GROUP BY customer_id
	), company_stats AS (
		SELECT coalesce(percentile_cont(0.5) WITHIN GROUP(ORDER BY median_interval) FILTER(WHERE median_interval IS NOT NULL),30) fallback_interval,
			extract(epoch FROM(max(last_purchase)-min(first_purchase)))/86400.0 data_days FROM summaries
	), scored AS (
		SELECT s.*,greatest(0,floor(extract(epoch FROM(now()-last_purchase))/86400.0))::int days_since,
			coalesce(median_interval,c.fallback_interval) expected_interval,c.data_days,
			ntile(5) OVER(ORDER BY last_purchase) r,ntile(5) OVER(ORDER BY purchase_count) f,ntile(5) OVER(ORDER BY lifetime_revenue) m
		FROM summaries s CROSS JOIN company_stats c
	), referrals AS (
		SELECT referrer_customer_id,count(*) FILTER(WHERE a.status IN('qualified','reward_pending','rewarded')) referral_count,
			coalesce(sum(t.net_amount),0) referral_revenue FROM referral_attributions a LEFT JOIN sales_transactions t ON t.id=a.qualifying_transaction_id
		WHERE a.company_id=$1 GROUP BY referrer_customer_id
	) INSERT INTO analytics_customer_features(company_id,customer_id,first_purchase_at,last_purchase_at,purchase_count,lifetime_revenue,average_check,days_since_last_purchase,median_purchase_interval_days,recency_score,frequency_score,monetary_score,rfm_segment,churn_risk,predicted_ltv,referral_count,referral_revenue)
	SELECT $1,s.customer_id,s.first_purchase,s.last_purchase,s.purchase_count,s.lifetime_revenue,s.average_check,s.days_since,s.expected_interval,s.r,s.f,s.m,
		CASE WHEN r>=4 AND f>=4 AND m>=4 THEN 'champions' WHEN r>=3 AND f>=4 THEN 'loyal' WHEN r>=4 AND f BETWEEN 2 AND 3 THEN 'potential_loyalist' WHEN r=5 AND purchase_count=1 THEN 'new' WHEN r BETWEEN 2 AND 3 AND f>=2 THEN 'need_attention' WHEN r<=2 AND f>=3 THEN 'at_risk' ELSE 'lost' END,
		CASE WHEN days_since<1.2*expected_interval THEN 'low' WHEN days_since<=2*expected_interval THEN 'medium' ELSE 'high' END,
		CASE WHEN data_days>=180 THEN (average_check::double precision*(30.0/greatest(expected_interval,1.0))*12.0)::numeric ELSE NULL END,
		coalesce(ref.referral_count,0),coalesce(ref.referral_revenue,0)
	FROM scored s LEFT JOIN referrals ref ON ref.referrer_customer_id=s.customer_id`, companyID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *api) refreshAnalytics(w http.ResponseWriter, r *http.Request) {
	if err := refreshAnalyticsCompany(r.Context(), a.db, companyID(r)); err != nil {
		slog.Warn("manual analytics refresh failed", "company", companyID(r), "error", err)
		fail(w, 500, "ANALYTICS_REFRESH_FAILED", "Не удалось обновить аналитические проекции")
		return
	}
	write(w, 202, envelope{Success: true, Data: map[string]any{"refreshed": true, "completedAt": time.Now().UTC()}})
}
