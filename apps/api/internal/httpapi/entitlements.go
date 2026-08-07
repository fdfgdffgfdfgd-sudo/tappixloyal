package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type entitlementSnapshot struct {
	Enabled bool
	Limit   *int
	Used    int
}

func (a *api) entitlement(ctx context.Context, company, code string) entitlementSnapshot {
	result := entitlementSnapshot{}
	_ = a.db.QueryRow(ctx, `SELECT coalesce(o.enabled,e.enabled,false),coalesce(o.limit_value,e.limit_value) FROM subscriptions s LEFT JOIN plan_entitlements e ON e.plan_code=CASE lower(s.plan_code) WHEN 'business' THEN 'growth' WHEN 'enterprise' THEN 'pro' ELSE lower(s.plan_code) END AND e.code=$2 LEFT JOIN LATERAL (SELECT enabled,limit_value FROM subscription_overrides WHERE company_id=s.company_id AND entitlement_code=$2 AND (valid_until IS NULL OR valid_until>now()) ORDER BY created_at DESC LIMIT 1) o ON true WHERE s.company_id=$1 AND s.status IN('trial','active','past_due') ORDER BY s.created_at DESC LIMIT 1`, company, code).Scan(&result.Enabled, &result.Limit)
	switch code {
	case "customers":
		_ = a.db.QueryRow(ctx, `SELECT count(*) FROM customers WHERE company_id=$1 AND deleted_at IS NULL`, company).Scan(&result.Used)
	case "staff":
		_ = a.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE company_id=$1 AND role='employee' AND deleted_at IS NULL`, company).Scan(&result.Used)
	case "smart_links":
		_ = a.db.QueryRow(ctx, `SELECT count(*) FROM devices WHERE company_id=$1`, company).Scan(&result.Used)
	case "messages_monthly":
		_ = a.db.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE company_id=$1 AND created_at>=date_trunc('month',now())`, company).Scan(&result.Used)
	}
	return result
}

func (a *api) checkLimit(ctx context.Context, company, code string) (bool, entitlementSnapshot) {
	s := a.entitlement(ctx, company, code)
	return s.Enabled && (s.Limit == nil || s.Used < *s.Limit), s
}

func (a *api) requireWritableSubscription(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodOptions || strings.HasPrefix(r.URL.Path, "/api/v1/workspaces") {
			next.ServeHTTP(w, r)
			return
		}
		claims, _ := r.Context().Value(identityKey).(tokenClaims)
		if claims.Role == "super_admin" || claims.CompanyID == "" {
			next.ServeHTTP(w, r)
			return
		}
		var status string
		var ends *time.Time
		err := a.db.QueryRow(r.Context(), `SELECT status,current_period_ends_at FROM subscriptions WHERE company_id=$1 ORDER BY created_at DESC LIMIT 1`, claims.CompanyID).Scan(&status, &ends)
		if err != nil || status == "cancelled" || status == "expired" || (ends != nil && ends.Before(time.Now()) && status != "past_due") {
			fail(w, 402, "SUBSCRIPTION_EXPIRED", "Подписка завершена. Данные доступны только для просмотра")
			return
		}
		next.ServeHTTP(w, r)
	})
}
