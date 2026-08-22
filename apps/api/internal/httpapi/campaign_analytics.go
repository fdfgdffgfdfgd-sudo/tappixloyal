package httpapi

import (
	"math"
	"net/http"
	"strings"
	"time"
)

type campaignEventInput struct {
	CustomerID     string  `json:"customerId"`
	ConversionType string  `json:"conversionType"`
	Value          float64 `json:"value"`
	IdempotencyKey string  `json:"idempotencyKey"`
}

func (a *api) recordCampaignEvent(w http.ResponseWriter, r *http.Request) {
	var in campaignEventInput
	if !decode(w, r, &in) {
		return
	}
	in.ConversionType = strings.ToLower(strings.TrimSpace(in.ConversionType))
	if in.CustomerID == "" || (in.ConversionType != "delivered" && in.ConversionType != "opened" && in.ConversionType != "clicked" && in.ConversionType != "redeemed") || in.Value < 0 || strings.TrimSpace(in.IdempotencyKey) == "" {
		fail(w, 422, "INVALID_CAMPAIGN_EVENT", "Укажите клиента, тип события и idempotencyKey")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось записать событие")
		return
	}
	defer tx.Rollback(r.Context())
	var recipientID string
	err = tx.QueryRow(r.Context(), `SELECT id FROM campaign_recipients WHERE company_id=$1 AND campaign_id=$2 AND customer_id=$3 AND experiment_group='treatment'`, companyID(r), r.PathValue("id"), in.CustomerID).Scan(&recipientID)
	if err != nil {
		fail(w, 404, "CAMPAIGN_RECIPIENT_NOT_FOUND", "Получатель кампании не найден")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO campaign_conversions(company_id,campaign_id,campaign_recipient_id,customer_id,conversion_type,conversion_value,idempotency_key)
		VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, companyID(r), r.PathValue("id"), recipientID, in.CustomerID, in.ConversionType, in.Value, in.IdempotencyKey)
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE campaign_recipients SET delivered_at=CASE WHEN $4='delivered' THEN coalesce(delivered_at,now()) ELSE delivered_at END,
			opened_at=CASE WHEN $4='opened' THEN coalesce(opened_at,now()) ELSE opened_at END,clicked_at=CASE WHEN $4='clicked' THEN coalesce(clicked_at,now()) ELSE clicked_at END
			WHERE company_id=$1 AND campaign_id=$2 AND id=$3`, companyID(r), r.PathValue("id"), recipientID, in.ConversionType)
	}
	if err == nil && (in.ConversionType == "delivered" || in.ConversionType == "opened") {
		eventType := "campaign.sent"
		if in.ConversionType == "opened" {
			eventType = "campaign.opened"
		}
		err = appendCustomerEvent(r, tx, companyID(r), in.CustomerID, eventType, "", "campaign-event:"+in.IdempotencyKey, map[string]any{"campaignId": r.PathValue("id"), "conversionType": in.ConversionType})
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "CAMPAIGN_EVENT_FAILED", "Не удалось записать событие")
		return
	}
	write(w, 202, envelope{Success: true, Data: map[string]any{"recorded": true, "conversionType": in.ConversionType}})
}

func (a *api) campaignAnalytics(w http.ResponseWriter, r *http.Request) {
	var name, status string
	var sentAt *time.Time
	var messageCost, rewardCost float64
	var windowDays, holdoutPercent int
	err := a.db.QueryRow(r.Context(), `SELECT name,status,sent_at,message_cost,reward_cost,attribution_window_days,holdout_percent FROM marketing_campaigns WHERE company_id=$1 AND id=$2`, companyID(r), r.PathValue("id")).Scan(&name, &status, &sentAt, &messageCost, &rewardCost, &windowDays, &holdoutPercent)
	if err != nil {
		fail(w, 404, "CAMPAIGN_NOT_FOUND", "Кампания не найдена")
		return
	}
	var treatment, holdout, delivered, opened, clicked, treatmentBuyers, holdoutBuyers, redemptions int
	var treatmentRevenue, holdoutRevenue, attributedRevenue float64
	err = a.db.QueryRow(r.Context(), `WITH recipients AS (
		SELECT r.*,c.sent_at campaign_sent_at,c.attribution_window_days FROM campaign_recipients r JOIN marketing_campaigns c ON c.id=r.campaign_id
		WHERE r.company_id=$1 AND r.campaign_id=$2
	), purchases AS (
		SELECT r.experiment_group,count(DISTINCT t.customer_id) FILTER(WHERE t.id IS NOT NULL) buyers,
			coalesce(sum(greatest(0,t.net_amount-coalesce((SELECT sum(rt.net_amount) FROM sales_transactions rt WHERE rt.company_id=t.company_id AND rt.original_transaction_id=t.id),0))),0) revenue
		FROM recipients r LEFT JOIN sales_transactions t ON t.company_id=r.company_id AND t.customer_id=r.customer_id AND t.original_transaction_id IS NULL
			AND t.status IN('completed','partially_refunded','refunded') AND NOT t.sandbox AND t.occurred_at>=r.campaign_sent_at
			AND t.occurred_at<=r.campaign_sent_at+make_interval(days=>r.attribution_window_days)
		GROUP BY r.experiment_group
	), conversions AS (
		SELECT coalesce((SELECT sum(greatest(0,t.net_amount-coalesce((SELECT sum(rt.net_amount) FROM sales_transactions rt WHERE rt.company_id=t.company_id AND rt.original_transaction_id=t.id),0))) FROM sales_transactions t WHERE t.company_id=$1 AND t.campaign_id=$2 AND t.original_transaction_id IS NULL AND NOT t.sandbox),0) attributed,
			count(*) FILTER(WHERE conversion_type='redeemed') redeemed FROM campaign_conversions WHERE company_id=$1 AND campaign_id=$2
	), recipient_stats AS (
		SELECT count(*) FILTER(WHERE experiment_group='treatment') treatment,count(*) FILTER(WHERE experiment_group='holdout') holdout,
			count(*) FILTER(WHERE delivered_at IS NOT NULL) delivered,count(*) FILTER(WHERE opened_at IS NOT NULL) opened,count(*) FILTER(WHERE clicked_at IS NOT NULL) clicked FROM recipients
	) SELECT rs.treatment,rs.holdout,rs.delivered,rs.opened,rs.clicked,
		coalesce((SELECT buyers FROM purchases WHERE experiment_group='treatment'),0),coalesce((SELECT revenue FROM purchases WHERE experiment_group='treatment'),0),
		coalesce((SELECT buyers FROM purchases WHERE experiment_group='holdout'),0),coalesce((SELECT revenue FROM purchases WHERE experiment_group='holdout'),0),c.attributed,c.redeemed
	FROM recipient_stats rs CROSS JOIN conversions c`, companyID(r), r.PathValue("id")).Scan(&treatment, &holdout, &delivered, &opened, &clicked, &treatmentBuyers, &treatmentRevenue, &holdoutBuyers, &holdoutRevenue, &attributedRevenue, &redemptions)
	if err != nil {
		fail(w, 500, "CAMPAIGN_ANALYTICS_FAILED", "Не удалось рассчитать результаты кампании")
		return
	}
	messageSpend := float64(delivered) * messageCost
	rewardSpend := float64(redemptions) * rewardCost
	totalCost := messageSpend + rewardSpend
	treatmentRate, holdoutRate := percentage(treatmentBuyers, treatment), percentage(holdoutBuyers, holdout)
	upliftConfigured := holdoutPercent > 0 && holdout > 0
	var observedPurchases int
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM sales_transactions t JOIN campaign_recipients r ON r.company_id=t.company_id AND r.customer_id=t.customer_id AND r.campaign_id=$2
		JOIN marketing_campaigns c ON c.id=r.campaign_id AND c.company_id=r.company_id
		WHERE t.company_id=$1 AND t.status IN('completed','partially_refunded') AND NOT t.sandbox AND t.occurred_at>=c.sent_at AND t.occurred_at<=c.sent_at+make_interval(days=>c.attribution_window_days)`, companyID(r), r.PathValue("id")).Scan(&observedPurchases)
	upliftAvailable := upliftConfigured && observedPurchases > 0
	var uplift, incrementalRevenue any
	var confidence any
	significant := false
	revenueForROI := attributedRevenue
	revenueLabel := "attributed_revenue"
	if upliftAvailable {
		uplift = treatmentRate - holdoutRate
		controlAverage := 0.0
		if holdout > 0 {
			controlAverage = holdoutRevenue / float64(holdout)
		}
		incremental := treatmentRevenue - controlAverage*float64(treatment)
		incrementalRevenue = incremental
		revenueForROI = incremental
		revenueLabel = "incremental_revenue"
		if treatment >= 30 && holdout >= 30 {
			p1 := float64(treatmentBuyers) / float64(treatment)
			p2 := float64(holdoutBuyers) / float64(holdout)
			pooled := float64(treatmentBuyers+holdoutBuyers) / float64(treatment+holdout)
			standardError := math.Sqrt(pooled * (1 - pooled) * (1/float64(treatment) + 1/float64(holdout)))
			if standardError > 0 {
				z := math.Abs(p1-p2) / standardError
				confidence = (2*0.5*(1+math.Erf(z/math.Sqrt2)) - 1) * 100
				significant = z >= 1.96
			}
		}
	}
	var roi any
	revenueAvailable := (upliftAvailable && (treatmentRevenue > 0 || holdoutRevenue > 0)) || (!upliftAvailable && attributedRevenue > 0)
	if totalCost > 0 && revenueAvailable {
		roi = (revenueForROI - totalCost) / totalCost * 100
	}
	roiUnavailableReason := ""
	if !revenueAvailable {
		roiUnavailableReason = "Подключите POS и накопите данные об атрибутированных продажах"
	} else if totalCost <= 0 {
		roiUnavailableReason = "Укажите стоимость сообщений и наград"
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{
		"campaignId": r.PathValue("id"), "name": name, "status": status, "sentAt": sentAt,
		"attributionWindowDays": windowDays, "holdoutPercent": holdoutPercent, "upliftConfigured": upliftConfigured, "upliftAvailable": upliftAvailable,
		"upliftUnavailableReason": func() string {
			if !upliftConfigured {
				return "Для расчёта uplift требуется контрольная группа"
			}
			if observedPurchases == 0 {
				return "Подключите POS и накопите продажи основной и контрольной групп"
			}
			return ""
		}(),
		"experiment": map[string]any{"sampleSufficient": treatment >= 30 && holdout >= 30, "statisticallySignificant": significant, "confidencePercent": confidence},
		"audience":   map[string]int{"treatment": treatment, "holdout": holdout},
		"delivery":   map[string]any{"delivered": delivered, "opened": opened, "clicked": clicked, "openRate": percentage(opened, delivered), "clickRate": percentage(clicked, delivered)},
		"purchases":  map[string]any{"treatmentBuyers": treatmentBuyers, "treatmentRevenue": treatmentRevenue, "holdoutBuyers": holdoutBuyers, "holdoutRevenue": holdoutRevenue, "treatmentConversionRate": treatmentRate, "holdoutConversionRate": holdoutRate, "attributedRevenue": attributedRevenue, "incrementalRevenue": incrementalRevenue, "upliftPercentagePoints": uplift},
		"costs":      map[string]float64{"messages": messageSpend, "rewards": rewardSpend, "total": totalCost},
		"roi":        roi, "roiAvailable": revenueAvailable && totalCost > 0, "roiUnavailableReason": roiUnavailableReason, "roiRevenueBasis": revenueLabel,
	}})
}
