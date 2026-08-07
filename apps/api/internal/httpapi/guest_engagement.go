package httpapi

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"
)

func levelProgress(points int) map[string]any {
	levels := []struct {
		Name string
		Min  int
	}{{"Bronze", 0}, {"Silver", 500}, {"Gold", 1000}, {"Platinum", 2500}, {"Diamond", 5000}}
	current, next := levels[0], levels[len(levels)-1]
	for i, l := range levels {
		if points >= l.Min {
			current = l
			if i+1 < len(levels) {
				next = levels[i+1]
			} else {
				next = l
			}
		}
	}
	progress := 100
	if next.Min > current.Min {
		progress = (points - current.Min) * 100 / (next.Min - current.Min)
		if progress > 100 {
			progress = 100
		}
	}
	return map[string]any{"current": current.Name, "next": next.Name, "currentMin": current.Min, "nextMin": next.Min, "progress": progress, "remaining": max(0, next.Min-points)}
}

func (a *api) customerWallet(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var points, visits, monthVisits, earned, spent int
	var referral string
	err := a.db.QueryRow(r.Context(), `SELECT c.total_points,c.total_visits,coalesce(c.referral_code,''),(SELECT count(*) FROM visits v WHERE v.company_id=c.company_id AND v.customer_id=c.id AND date_trunc('month',v.created_at)=date_trunc('month',now())),(SELECT coalesce(sum(amount),0) FROM bonus_ledger b WHERE b.company_id=c.company_id AND b.customer_id=c.id AND b.operation='credit' AND date_trunc('month',b.created_at)=date_trunc('month',now())),(SELECT coalesce(sum(amount),0) FROM bonus_ledger b WHERE b.company_id=c.company_id AND b.customer_id=c.id AND b.operation='debit' AND date_trunc('month',b.created_at)=date_trunc('month',now())) FROM customers c WHERE c.company_id=$1 AND c.id=$2`, claims.CompanyID, claims.Subject).Scan(&points, &visits, &referral, &monthVisits, &earned, &spent)
	if err != nil {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Карта не найдена")
		return
	}
	achievements := []map[string]any{{"code": "joined", "title": "Участник клуба", "description": "Цифровая карта активирована", "unlocked": true}, {"code": "first_visit", "title": "Первое посещение", "description": "Первый шаг сделан", "unlocked": visits >= 1}, {"code": "regular", "title": "Постоянный клиент", "description": "5 посещений", "unlocked": visits >= 5}, {"code": "favorite", "title": "Любимый клиент", "description": "10 посещений", "unlocked": visits >= 10}, {"code": "vip", "title": "VIP", "description": "Уровень Platinum", "unlocked": points >= 2500}, {"code": "ambassador", "title": "Амбассадор", "description": "Приглашён первый друг", "unlocked": false}}
	visitsTarget := 5
	if visits >= 5 {
		visitsTarget = 10
	}
	if visits >= 10 {
		visitsTarget = 25
	}
	var lastSpin *time.Time
	_ = a.db.QueryRow(r.Context(), `SELECT max(created_at) FROM customer_wheel_spins WHERE company_id=$1 AND customer_id=$2`, claims.CompanyID, claims.Subject).Scan(&lastSpin)
	canSpin := lastSpin == nil || lastSpin.Before(time.Now().Add(-7*24*time.Hour))
	write(w, 200, envelope{Success: true, Data: map[string]any{"level": levelProgress(points), "monthly": map[string]int{"visits": monthVisits, "earned": earned, "spent": spent, "savings": spent * 10}, "achievements": achievements, "nextReward": map[string]any{"title": fmt.Sprintf("Подарок за %d посещений", visitsTarget), "remaining": max(0, visitsTarget-visits), "target": visitsTarget}, "referralCode": referral, "referralUrl": fmt.Sprintf("%s/r/%s", envOr("APP_URL", "http://localhost:8088"), referral), "walletPassStatus": "planned", "wheel": map[string]any{"canSpin": canSpin, "lastSpin": lastSpin}}})
}

func (a *api) customerWheelSpin(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var recent bool
	_ = a.db.QueryRow(r.Context(), `SELECT exists(SELECT 1 FROM customer_wheel_spins WHERE company_id=$1 AND customer_id=$2 AND created_at>now()-interval '7 days')`, claims.CompanyID, claims.Subject).Scan(&recent)
	if recent {
		fail(w, 409, "SPIN_NOT_AVAILABLE", "Следующая попытка будет доступна через неделю")
		return
	}
	buf := []byte{0}
	_, _ = rand.Read(buf)
	prizes := []struct {
		Value int
		Label string
	}{{20, "20 бонусов"}, {20, "20 бонусов"}, {50, "50 бонусов"}, {0, "Двойные бонусы"}, {0, "Подарок от компании"}}
	prize := prizes[int(buf[0])%len(prizes)]
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось запустить колесо")
		return
	}
	defer tx.Rollback(r.Context())
	var balance int
	if prize.Value > 0 {
		err = tx.QueryRow(r.Context(), `UPDATE customers SET total_points=total_points+$3,updated_at=now() WHERE company_id=$1 AND id=$2 RETURNING total_points`, claims.CompanyID, claims.Subject, prize.Value).Scan(&balance)
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description) VALUES($1,$2,'credit',$3,$4,'Приз: счастливое колесо')`, claims.CompanyID, claims.Subject, prize.Value, balance)
		}
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO customer_wheel_spins(company_id,customer_id,prize_type,prize_value,prize_label) VALUES($1,$2,$3,$4,$5)`, claims.CompanyID, claims.Subject, "reward", prize.Value, prize.Label)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "SPIN_FAILED", "Не удалось сохранить приз")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"label": prize.Label, "points": prize.Value, "balance": balance}})
}

func (a *api) publicReferral(w http.ResponseWriter, r *http.Request) {
	var token, company string
	err := a.db.QueryRow(r.Context(), `SELECT d.token,co.name FROM customers c JOIN companies co ON co.id=c.company_id AND co.status='active' JOIN LATERAL (SELECT token FROM devices WHERE company_id=c.company_id AND is_active AND destination='join' ORDER BY created_at LIMIT 1) d ON true WHERE c.referral_code=$1 AND c.deleted_at IS NULL`, r.PathValue("code")).Scan(&token, &company)
	if err != nil {
		fail(w, 404, "REFERRAL_NOT_FOUND", "Приглашение недействительно")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]string{"token": token, "company": company, "referralCode": r.PathValue("code")}})
}
