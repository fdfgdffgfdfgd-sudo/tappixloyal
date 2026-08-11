package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func (a *api) bonusLiability(w http.ResponseWriter, r *http.Request) {
	var issued, active, redeemed, expired, liability, expected float64
	err := a.db.QueryRow(r.Context(), `WITH lots AS (
		SELECT coalesce(sum(issued_amount),0)::numeric issued,
		coalesce(sum(remaining_amount) FILTER(WHERE status='active' AND activates_at<=now() AND (expires_at IS NULL OR expires_at>now())),0)::numeric active,
		coalesce(sum((remaining_amount::numeric/issued_amount)*monetary_value) FILTER(WHERE status IN('pending','active') AND (expires_at IS NULL OR expires_at>now())),0) liability
		FROM bonus_lots WHERE company_id=$1
	), redemptions AS (SELECT coalesce(sum(redeemed_amount-restored_amount),0)::numeric redeemed FROM bonus_lot_redemptions WHERE company_id=$1),
	expirations AS (SELECT coalesce(sum(amount),0)::numeric expired FROM bonus_ledger WHERE company_id=$1 AND operation='expire')
	SELECT l.issued,l.active,r.redeemed,e.expired,l.liability,
		CASE WHEN l.issued=0 THEN 0 ELSE l.liability*least(1,r.redeemed/l.issued) END
	FROM lots l CROSS JOIN redemptions r CROSS JOIN expirations e`, companyID(r)).Scan(&issued, &active, &redeemed, &expired, &liability, &expected)
	if err != nil {
		fail(w, 500, "BONUS_LIABILITY_FAILED", "Не удалось рассчитать бонусные обязательства")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"currency": "KZT", "issued": issued, "active": active, "redeemed": redeemed, "expired": expired, "liability": liability, "expectedRedemptionCost": expected}})
}

func expireBonusLots(ctx context.Context, db *pgxpool.Pool, tenant string) (int64, error) {
	query := `WITH expired AS (
		SELECT company_id,customer_id,sum(remaining_amount)::integer amount FROM bonus_lots
		WHERE status IN('pending','active') AND remaining_amount>0 AND expires_at<=now() AND ($1='' OR company_id=$1::uuid)
		GROUP BY company_id,customer_id
	), adjusted AS (
		UPDATE customers c SET total_points=greatest(0,c.total_points-e.amount),updated_at=now() FROM expired e
		WHERE c.company_id=e.company_id AND c.id=e.customer_id RETURNING c.company_id,c.id customer_id,least(e.amount,c.total_points+e.amount) amount,c.total_points balance_after
	), ledger AS (
		INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description,idempotency_key)
		SELECT company_id,customer_id,'expire',amount,balance_after,'Автоматическое сгорание бонусов','bonus-expiry:'||customer_id||':'||to_char(now(),'YYYY-MM-DD') FROM adjusted WHERE amount>0
		ON CONFLICT(company_id,idempotency_key) DO NOTHING RETURNING 1
	) UPDATE bonus_lots SET remaining_amount=0,status='expired',updated_at=now()
	WHERE status IN('pending','active') AND remaining_amount>0 AND expires_at<=now() AND ($1='' OR company_id=$1::uuid)`
	tag, err := db.Exec(ctx, query, tenant)
	return tag.RowsAffected(), err
}

func (a *api) expireBonusLotsNow(w http.ResponseWriter, r *http.Request) {
	count, err := expireBonusLots(r.Context(), a.db, companyID(r))
	if err != nil {
		fail(w, 500, "BONUS_EXPIRY_FAILED", "Не удалось обработать сгорание бонусов")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"expiredLots": count, "completedAt": time.Now().UTC()}})
}
