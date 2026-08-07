package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func StartAutomation(ctx context.Context, db *pgxpool.Pool) {
	run := func() { _, _ = processBirthdayBonuses(ctx, db, "") }
	go func() {
		run()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func processBirthdayBonuses(ctx context.Context, db *pgxpool.Pool, onlyCompany string) (int, error) {
	query := `SELECT c.id,c.company_id,coalesce((lr.actions->>'amount')::integer,0) FROM customers c JOIN companies co ON co.id=c.company_id AND co.status='active' JOIN loyalty_rules lr ON lr.company_id=c.company_id AND lr.event_type='customer_birthday' AND lr.is_active WHERE c.deleted_at IS NULL AND c.birthday IS NOT NULL AND extract(month from c.birthday)=extract(month from current_date) AND extract(day from c.birthday)=extract(day from current_date) AND ($1='' OR c.company_id::text=$1)`
	rows, err := db.Query(ctx, query, onlyCompany)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type item struct {
		id, company string
		amount      int
	}
	items := []item{}
	for rows.Next() {
		var x item
		if rows.Scan(&x.id, &x.company, &x.amount) == nil && x.amount > 0 {
			items = append(items, x)
		}
	}
	processed := 0
	year := time.Now().UTC().Year()
	for _, x := range items {
		tx, e := db.Begin(ctx)
		if e != nil {
			slog.Warn("birthday automation transaction failed", "error", e)
			continue
		}
		key := fmt.Sprintf("birthday:%s:%d", x.id, year)
		var eventID string
		e = tx.QueryRow(ctx, `INSERT INTO loyalty_events(company_id,customer_id,event_type,event_key,payload) VALUES($1,$2,'customer_birthday',$3,jsonb_build_object('amount',$4::integer)) ON CONFLICT(company_id,event_key) DO NOTHING RETURNING id`, x.company, x.id, key, x.amount).Scan(&eventID)
		if errors.Is(e, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			continue
		}
		if e != nil {
			slog.Warn("birthday automation event failed", "customer", x.id, "error", e)
			_ = tx.Rollback(ctx)
			continue
		}
		var balance int
		e = tx.QueryRow(ctx, `UPDATE customers SET total_points=total_points+$3,updated_at=now() WHERE company_id=$1 AND id=$2 RETURNING total_points`, x.company, x.id, x.amount).Scan(&balance)
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description,idempotency_key) VALUES($1,$2,'credit',$3,$4,'Бонус на день рождения',$5)`, x.company, x.id, x.amount, balance, key)
		}
		if e == nil && tx.Commit(ctx) == nil {
			processed++
		} else {
			slog.Warn("birthday automation credit failed", "customer", x.id, "error", e)
			_ = tx.Rollback(ctx)
		}
	}
	return processed, nil
}

func (a *api) processBirthdays(w http.ResponseWriter, r *http.Request) {
	count, err := processBirthdayBonuses(r.Context(), a.db, companyID(r))
	if err != nil {
		fail(w, 500, "AUTOMATION_FAILED", "Не удалось обработать birthday-бонусы")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]int{"processed": count}})
}

func (a *api) inactiveCustomers(w http.ResponseWriter, r *http.Request) {
	days := clamp(parseInt(r.URL.Query().Get("days"), 30), 1, 3650)
	rows, err := a.db.Query(r.Context(), `SELECT c.id,c.first_name,c.last_name,c.phone,c.total_points,c.total_visits,max(v.created_at) FROM customers c LEFT JOIN visits v ON v.company_id=c.company_id AND v.customer_id=c.id WHERE c.company_id=$1 AND c.deleted_at IS NULL GROUP BY c.id HAVING coalesce(max(v.created_at),c.created_at)<now()-make_interval(days=>$2::integer) ORDER BY max(v.created_at) NULLS FIRST LIMIT 500`, companyID(r), days)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось найти неактивных клиентов")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, first, last, phone string
		var points, visits int
		var lastVisit *time.Time
		if rows.Scan(&id, &first, &last, &phone, &points, &visits, &lastVisit) == nil {
			items = append(items, map[string]any{"id": id, "firstName": first, "lastName": last, "phone": phone, "points": points, "visits": visits, "lastVisitAt": lastVisit})
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"days": days, "items": items, "total": len(items)}})
}
