package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (a *api) pointsForEvent(ctx context.Context, companyID, event string, fallback int) int {
	var amount int
	err := a.db.QueryRow(ctx, `SELECT coalesce((actions->>'amount')::integer,0) FROM loyalty_rules WHERE company_id=$1 AND event_type=$2 AND is_active ORDER BY priority LIMIT 1`, companyID, event).Scan(&amount)
	if err != nil || amount < 0 {
		return fallback
	}
	return amount
}

type branchInput struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
}
type moduleInput struct {
	Enabled bool `json:"enabled"`
}
type loyaltyInput struct {
	WelcomeBonus    int    `json:"welcomeBonus"`
	PointsPerVisit  int    `json:"pointsPerVisit"`
	BirthdayBonus   int    `json:"birthdayBonus"`
	VisitsForReward int    `json:"visitsForReward"`
	RewardName      string `json:"rewardName"`
}

func (a *api) createBranch(w http.ResponseWriter, r *http.Request) {
	var in branchInput
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Address = strings.TrimSpace(in.Address)
	if in.Name == "" || in.Address == "" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите название и адрес филиала")
		return
	}
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO branches(company_id,name,address,phone) VALUES($1,$2,$3,$4) RETURNING id`, companyID(r), in.Name, in.Address, in.Phone).Scan(&id)
	if err != nil {
		fail(w, 409, "BRANCH_EXISTS", "Филиал с таким названием уже существует")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": id, "name": in.Name, "address": in.Address, "phone": in.Phone}})
}
func (a *api) updateBranch(w http.ResponseWriter, r *http.Request) {
	var in branchInput
	if !decode(w, r, &in) {
		return
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE branches SET name=coalesce(nullif($3,''),name),address=coalesce(nullif($4,''),address),phone=$5,updated_at=now() WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), r.PathValue("id"), strings.TrimSpace(in.Name), strings.TrimSpace(in.Address), in.Phone)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "BRANCH_NOT_FOUND", "Филиал не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"updated": true}})
}
func (a *api) deleteBranch(w http.ResponseWriter, r *http.Request) {
	tag, err := a.db.Exec(r.Context(), `UPDATE branches SET deleted_at=now(),is_active=false WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "BRANCH_NOT_FOUND", "Филиал не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"archived": true}})
}
func (a *api) updateModule(w http.ResponseWriter, r *http.Request) {
	var in moduleInput
	if !decode(w, r, &in) {
		return
	}
	code := r.PathValue("code")
	var core bool
	err := a.db.QueryRow(r.Context(), `SELECT is_core FROM modules WHERE code=$1`, code).Scan(&core)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "MODULE_NOT_FOUND", "Модуль не найден")
		return
	}
	if core && !in.Enabled {
		fail(w, 409, "CORE_MODULE_REQUIRED", "Core нельзя отключить")
		return
	}
	_, err = a.db.Exec(r.Context(), `INSERT INTO company_modules(company_id,module_code,enabled) VALUES($1,$2,$3) ON CONFLICT(company_id,module_code) DO UPDATE SET enabled=excluded.enabled,updated_at=now()`, companyID(r), code, in.Enabled)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось изменить модуль")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"code": code, "enabled": in.Enabled || core}})
}

func (a *api) moduleIncluded(ctx context.Context, company, code string) bool {
	if code == "core" || code == "crm" || code == "loyalty" || code == "reviews" {
		return true
	}
	var plan string
	if a.db.QueryRow(ctx, `SELECT lower(plan_code) FROM subscriptions WHERE company_id=$1 AND status IN('trial','active','past_due') ORDER BY created_at DESC LIMIT 1`, company).Scan(&plan) != nil {
		return false
	}
	if plan == "business" {
		plan = "growth"
	} else if plan == "enterprise" {
		plan = "pro"
	}
	if code == "api" {
		return plan == "pro"
	}
	return plan == "growth" || plan == "pro"
}

func (a *api) getLoyaltyRules(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT event_type,actions FROM loyalty_rules WHERE company_id=$1 AND is_active`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить правила")
		return
	}
	defer rows.Close()
	result := loyaltyInput{PointsPerVisit: 20}
	for rows.Next() {
		var event string
		var action map[string]any
		if rows.Scan(&event, &action) != nil {
			continue
		}
		amount, _ := action["amount"].(float64)
		switch event {
		case "customer_registered":
			result.WelcomeBonus = int(amount)
		case "visit_created":
			result.PointsPerVisit = int(amount)
		case "customer_birthday":
			result.BirthdayBonus = int(amount)
		case "visit_milestone":
			visits, _ := action["visits"].(float64)
			result.VisitsForReward = int(visits)
			result.RewardName, _ = action["rewardName"].(string)
		}
	}
	write(w, 200, envelope{Success: true, Data: result})
}
func (a *api) updateLoyaltyRules(w http.ResponseWriter, r *http.Request) {
	var in loyaltyInput
	if !decode(w, r, &in) {
		return
	}
	if in.WelcomeBonus < 0 || in.PointsPerVisit < 0 || in.BirthdayBonus < 0 || in.VisitsForReward < 0 {
		fail(w, 422, "VALIDATION_ERROR", "Значения не могут быть отрицательными")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сохранить правила")
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `DELETE FROM loyalty_rules WHERE company_id=$1`, companyID(r))
	rules := []struct {
		name, event string
		action      map[string]any
	}{{"Приветственный бонус", "customer_registered", map[string]any{"type": "credit_points", "amount": in.WelcomeBonus}}, {"Бонус за посещение", "visit_created", map[string]any{"type": "credit_points", "amount": in.PointsPerVisit}}, {"Бонус на день рождения", "customer_birthday", map[string]any{"type": "credit_points", "amount": in.BirthdayBonus}}, {"Подарок за посещения", "visit_milestone", map[string]any{"type": "reward", "visits": in.VisitsForReward, "rewardName": in.RewardName}}}
	for _, rule := range rules {
		if err != nil {
			break
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO loyalty_rules(company_id,name,event_type,actions) VALUES($1,$2,$3,$4)`, companyID(r), rule.name, rule.event, rule.action)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сохранить правила")
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}
