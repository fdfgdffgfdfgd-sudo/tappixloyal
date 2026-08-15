package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type rewardStatusInput struct {
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	BranchID           string `json:"branchId"`
	ReservationMinutes int    `json:"reservationMinutes"`
	IdempotencyKey     string `json:"idempotencyKey"`
}
type rewardDefinitionInput struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	RewardType         string   `json:"rewardType"`
	Value              int      `json:"value"`
	ValidityDays       *int     `json:"validityDays"`
	Repeatable         bool     `json:"repeatable"`
	CooldownDays       int      `json:"cooldownDays"`
	InventoryTotal     *int     `json:"inventoryTotal"`
	ConfirmationMethod string   `json:"confirmationMethod"`
	BranchIDs          []string `json:"branchIds"`
	Active             *bool    `json:"active"`
}
type rewardRuleInput struct {
	DefinitionID string `json:"definitionId"`
	EventType    string `json:"eventType"`
	Threshold    int    `json:"threshold"`
	ProgressMode string `json:"progressMode"`
	Priority     int    `json:"priority"`
	Active       *bool  `json:"active"`
}

func (a *api) rewardsFor(w http.ResponseWriter, r *http.Request, customerID string) {
	rows, err := a.db.Query(r.Context(), `SELECT cr.id,cr.name,cr.description,CASE WHEN cr.status IN ('available','reserved') AND cr.expires_at<=now() THEN 'expired' WHEN cr.status='reserved' AND cr.reserved_until<=now() THEN 'available' ELSE cr.status END,cr.issued_at,cr.expires_at,cr.reserved_until,cr.redeemed_at,coalesce(rd.reward_type,'gift'),coalesce(rd.value,0),cr.definition_id FROM customer_rewards cr LEFT JOIN reward_definitions rd ON rd.id=cr.definition_id WHERE cr.company_id=$1 AND cr.customer_id=$2 ORDER BY cr.issued_at DESC`, companyID(r), customerID)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить награды")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, description, status, kind string
		var issued time.Time
		var expires, reserved, redeemed *time.Time
		var value int
		var definitionID *string
		if rows.Scan(&id, &name, &description, &status, &issued, &expires, &reserved, &redeemed, &kind, &value, &definitionID) == nil {
			items = append(items, map[string]any{"id": id, "definitionId": definitionID, "name": name, "description": description, "status": status, "rewardType": kind, "value": value, "issuedAt": issued, "expiresAt": expires, "reservedUntil": reserved, "redeemedAt": redeemed})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) customerRewards(w http.ResponseWriter, r *http.Request) {
	a.rewardsFor(w, r, r.PathValue("id"))
}
func (a *api) customerOwnRewards(w http.ResponseWriter, r *http.Request) {
	a.rewardsFor(w, r, identity(r).Subject)
}

func (a *api) listRewardDefinitions(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT d.id,d.name,d.description,d.reward_type,d.value,d.validity_days,d.repeatable,d.cooldown_days,d.inventory_total,d.inventory_issued,d.confirmation_method,d.branch_ids,d.is_active,d.created_at,(SELECT count(*) FROM reward_rules rr WHERE rr.definition_id=d.id AND rr.is_active) FROM reward_definitions d WHERE d.company_id=$1 AND d.deleted_at IS NULL ORDER BY d.created_at DESC`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить каталог наград")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, description, kind, confirmation string
		var value, cooldown, issued, rules int
		var validity, inventory *int
		var repeatable, active bool
		var branches []string
		var created time.Time
		if rows.Scan(&id, &name, &description, &kind, &value, &validity, &repeatable, &cooldown, &inventory, &issued, &confirmation, &branches, &active, &created, &rules) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "description": description, "rewardType": kind, "value": value, "validityDays": validity, "repeatable": repeatable, "cooldownDays": cooldown, "inventoryTotal": inventory, "inventoryIssued": issued, "inventoryRemaining": remainingInventory(inventory, issued), "confirmationMethod": confirmation, "branchIds": branches, "active": active, "activeRules": rules, "createdAt": created})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func remainingInventory(total *int, issued int) any {
	if total == nil {
		return nil
	}
	n := *total - issued
	if n < 0 {
		n = 0
	}
	return n
}
func normalizeDefinition(in *rewardDefinitionInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.RewardType = strings.ToLower(in.RewardType)
	in.ConfirmationMethod = strings.ToLower(in.ConfirmationMethod)
	if in.RewardType == "" {
		in.RewardType = "gift"
	}
	if in.ConfirmationMethod == "" {
		in.ConfirmationMethod = "staff"
	}
	if in.Name == "" || in.Value < 0 || in.CooldownDays < 0 {
		return errors.New("invalid")
	}
	return nil
}
func (a *api) createRewardDefinition(w http.ResponseWriter, r *http.Request) {
	var in rewardDefinitionInput
	if !decode(w, r, &in) {
		return
	}
	if normalizeDefinition(&in) != nil {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте параметры награды")
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO reward_definitions(company_id,name,description,reward_type,value,validity_days,repeatable,cooldown_days,inventory_total,confirmation_method,branch_ids,is_active,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`, companyID(r), in.Name, strings.TrimSpace(in.Description), in.RewardType, in.Value, in.ValidityDays, in.Repeatable, in.CooldownDays, in.InventoryTotal, in.ConfirmationMethod, in.BranchIDs, active, identity(r).Subject).Scan(&id)
	if err != nil {
		fail(w, 422, "REWARD_DEFINITION_INVALID", "Не удалось создать награду")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": id}})
}
func (a *api) updateRewardDefinition(w http.ResponseWriter, r *http.Request) {
	var in rewardDefinitionInput
	if !decode(w, r, &in) {
		return
	}
	if normalizeDefinition(&in) != nil {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте параметры награды")
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE reward_definitions SET name=$3,description=$4,reward_type=$5,value=$6,validity_days=$7,repeatable=$8,cooldown_days=$9,inventory_total=$10,confirmation_method=$11,branch_ids=$12,is_active=$13,updated_at=now() WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), r.PathValue("id"), in.Name, strings.TrimSpace(in.Description), in.RewardType, in.Value, in.ValidityDays, in.Repeatable, in.CooldownDays, in.InventoryTotal, in.ConfirmationMethod, in.BranchIDs, active)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "REWARD_DEFINITION_NOT_FOUND", "Награда не найдена")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"updated": true}})
}
func (a *api) deleteRewardDefinition(w http.ResponseWriter, r *http.Request) {
	tag, err := a.db.Exec(r.Context(), `UPDATE reward_definitions SET is_active=false,deleted_at=now(),updated_at=now() WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "REWARD_DEFINITION_NOT_FOUND", "Награда не найдена")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"archived": true}})
}

func (a *api) listRewardRules(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT rr.id,rr.definition_id,rd.name,rr.event_type,rr.threshold,rr.progress_mode,rr.priority,rr.is_active,rr.created_at FROM reward_rules rr JOIN reward_definitions rd ON rd.id=rr.definition_id WHERE rr.company_id=$1 ORDER BY rr.priority,rr.created_at`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить правила")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, did, name, event, mode string
		var threshold, priority int
		var active bool
		var created time.Time
		if rows.Scan(&id, &did, &name, &event, &threshold, &mode, &priority, &active, &created) == nil {
			items = append(items, map[string]any{"id": id, "definitionId": did, "rewardName": name, "eventType": event, "threshold": threshold, "progressMode": mode, "priority": priority, "active": active, "createdAt": created})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) createRewardRule(w http.ResponseWriter, r *http.Request) {
	var in rewardRuleInput
	if !decode(w, r, &in) {
		return
	}
	if in.Threshold < 1 {
		in.Threshold = 1
	}
	if in.ProgressMode == "" {
		in.ProgressMode = "lifetime"
	}
	if in.Priority == 0 {
		in.Priority = 100
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO reward_rules(company_id,definition_id,event_type,threshold,progress_mode,priority,is_active) SELECT $1,d.id,$3,$4,$5,$6,$7 FROM reward_definitions d WHERE d.company_id=$1 AND d.id=$2 AND d.deleted_at IS NULL RETURNING id`, companyID(r), in.DefinitionID, in.EventType, in.Threshold, in.ProgressMode, in.Priority, active).Scan(&id)
	if err != nil {
		fail(w, 422, "REWARD_RULE_INVALID", "Проверьте правило и награду")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": id}})
}
func (a *api) updateRewardRule(w http.ResponseWriter, r *http.Request) {
	var in rewardRuleInput
	if !decode(w, r, &in) {
		return
	}
	if in.Threshold < 1 {
		fail(w, 422, "VALIDATION_ERROR", "Порог должен быть положительным")
		return
	}
	if in.ProgressMode == "" {
		in.ProgressMode = "lifetime"
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE reward_rules SET definition_id=$3,event_type=$4,threshold=$5,progress_mode=$6,priority=$7,is_active=$8,updated_at=now() WHERE company_id=$1 AND id=$2`, companyID(r), r.PathValue("id"), in.DefinitionID, in.EventType, in.Threshold, in.ProgressMode, in.Priority, active)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "REWARD_RULE_NOT_FOUND", "Правило не найдено")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"updated": true}})
}

func (a *api) customerRewardProgress(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT p.id,p.rule_id,rr.definition_id,rd.name,p.current_value,p.target_value,p.status,p.cycle_key,p.completed_at,p.updated_at FROM customer_reward_progress p JOIN reward_rules rr ON rr.id=p.rule_id JOIN reward_definitions rd ON rd.id=rr.definition_id WHERE p.company_id=$1 AND p.customer_id=$2 ORDER BY p.updated_at DESC`, companyID(r), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить прогресс")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, ruleID, definitionID, name, status, cycle string
		var current, target int
		var completed *time.Time
		var updated time.Time
		if rows.Scan(&id, &ruleID, &definitionID, &name, &current, &target, &status, &cycle, &completed, &updated) == nil {
			items = append(items, map[string]any{"id": id, "ruleId": ruleID, "definitionId": definitionID, "name": name, "currentValue": current, "targetValue": target, "status": status, "cycleKey": cycle, "completedAt": completed, "updatedAt": updated})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) issueReward(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CustomerID     string `json:"customerId"`
		DefinitionID   string `json:"definitionId"`
		Reason         string `json:"reason"`
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if !decode(w, r, &in) {
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать операцию")
		return
	}
	defer tx.Rollback(r.Context())
	id, err := a.issueRewardTx(r, tx, companyID(r), in.CustomerID, in.DefinitionID, "", nil, in.Reason, in.IdempotencyKey)
	if err != nil {
		rewardConflict(w, err)
		return
	}
	if tx.Commit(r.Context()) != nil {
		fail(w, 500, "REWARD_FAILED", "Не удалось выдать награду")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": id, "status": "available"}})
}

func (a *api) issueRewardTx(r *http.Request, tx pgx.Tx, tenant, customerID, definitionID, ruleID string, progressID *string, reason, idempotency string) (string, error) {
	if idempotency != "" {
		var existing string
		if err := tx.QueryRow(r.Context(), `SELECT id FROM customer_rewards WHERE company_id=$1 AND idempotency_key=$2`, tenant, idempotency).Scan(&existing); err == nil {
			return existing, nil
		}
	}
	var name, description string
	var validity *int
	var repeatable bool
	var cooldown int
	var inventory *int
	var issued int
	err := tx.QueryRow(r.Context(), `SELECT name,description,validity_days,repeatable,cooldown_days,inventory_total,inventory_issued FROM reward_definitions WHERE company_id=$1 AND id=$2 AND is_active AND deleted_at IS NULL FOR UPDATE`, tenant, definitionID).Scan(&name, &description, &validity, &repeatable, &cooldown, &inventory, &issued)
	if err != nil {
		return "", fmt.Errorf("definition")
	}
	if inventory != nil && issued >= *inventory {
		return "", fmt.Errorf("inventory")
	}
	if !repeatable {
		var exists bool
		_ = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM customer_rewards WHERE company_id=$1 AND customer_id=$2 AND definition_id=$3 AND status<>'cancelled')`, tenant, customerID, definitionID).Scan(&exists)
		if exists {
			return "", fmt.Errorf("repeat")
		}
	}
	if cooldown > 0 {
		var blocked bool
		_ = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM customer_rewards WHERE company_id=$1 AND customer_id=$2 AND definition_id=$3 AND issued_at>now()-make_interval(days=>$4))`, tenant, customerID, definitionID, cooldown).Scan(&blocked)
		if blocked {
			return "", fmt.Errorf("cooldown")
		}
	}
	var expires any
	if validity != nil {
		expires = time.Now().Add(time.Duration(*validity) * 24 * time.Hour)
	}
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO customer_rewards(company_id,customer_id,definition_id,rule_id,progress_id,name,description,status,expires_at,idempotency_key) SELECT $1,c.id,$3,nullif($4,'')::uuid,$5,$6,$7,'available',$8,nullif($9,'') FROM customers c WHERE c.company_id=$1 AND c.id=$2 AND c.deleted_at IS NULL ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO UPDATE SET idempotency_key=excluded.idempotency_key RETURNING id`, tenant, customerID, definitionID, ruleID, progressID, name, description, expires, idempotency).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("customer")
	}
	_, err = tx.Exec(r.Context(), `UPDATE reward_definitions SET inventory_issued=inventory_issued+1,updated_at=now() WHERE id=$1`, definitionID)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO reward_transactions(company_id,reward_id,customer_id,actor_id,operation,to_status,reason,idempotency_key) VALUES($1,$2,$3,$4,'issued','available',$5,nullif($6,'')) ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, tenant, id, customerID, identity(r).Subject, reason, idempotency)
	if err == nil {
		err = appendCustomerEvent(r, tx, tenant, customerID, "reward.unlocked", "", "reward-unlocked:"+id, map[string]any{"rewardId": id, "name": name, "description": description})
	}
	return id, err
}

func rewardConflict(w http.ResponseWriter, err error) {
	code := "REWARD_NOT_AVAILABLE"
	message := "Награду нельзя выдать"
	switch err.Error() {
	case "inventory":
		code = "REWARD_OUT_OF_STOCK"
		message = "Награды закончились"
	case "repeat":
		code = "REWARD_ALREADY_ISSUED"
		message = "Награда уже выдавалась"
	case "cooldown":
		code = "REWARD_COOLDOWN"
		message = "Период повторной выдачи ещё не завершён"
	case "definition":
		code = "REWARD_DEFINITION_NOT_FOUND"
		message = "Награда не найдена"
	case "customer":
		code = "CUSTOMER_NOT_FOUND"
		message = "Клиент не найден"
	}
	fail(w, 409, code, message)
}

func (a *api) transitionReward(w http.ResponseWriter, r *http.Request, target string) {
	var in rewardStatusInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.ReservationMinutes < 1 {
		in.ReservationMinutes = 15
	}
	if in.ReservationMinutes > 1440 {
		in.ReservationMinutes = 1440
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать операцию")
		return
	}
	defer tx.Rollback(r.Context())
	tenant := companyID(r)
	claims := identity(r)
	idempotency := strings.TrimSpace(in.IdempotencyKey)
	if idempotency != "" {
		idempotency = target + ":" + r.PathValue("id") + ":" + idempotency
		var existing string
		if tx.QueryRow(r.Context(), `SELECT reward_id FROM reward_transactions WHERE company_id=$1 AND idempotency_key=$2`, tenant, idempotency).Scan(&existing) == nil && existing == r.PathValue("id") {
			write(w, 200, envelope{Success: true, Data: map[string]any{"id": existing, "status": target, "idempotentReplay": true}})
			return
		}
	}
	var customerID, status string
	var expires, reservedUntil *time.Time
	err = tx.QueryRow(r.Context(), `SELECT customer_id,status,expires_at,reserved_until FROM customer_rewards WHERE company_id=$1 AND id=$2 FOR UPDATE`, tenant, r.PathValue("id")).Scan(&customerID, &status, &expires, &reservedUntil)
	if err != nil {
		fail(w, 404, "REWARD_NOT_FOUND", "Награда не найдена")
		return
	}
	now := time.Now()
	if expires != nil && !expires.After(now) {
		target = "expired"
	}
	status = effectiveRewardStatus(status, reservedUntil, now)
	valid := false
	switch target {
	case "reserved":
		valid = status == "available"
	case "redeemed":
		valid = status == "available" || (status == "reserved" && (reservedUntil == nil || reservedUntil.After(now)))
	case "cancelled":
		valid = status == "available" || status == "reserved"
	case "expired":
		valid = status == "available" || status == "reserved"
	}
	if !valid {
		fail(w, 409, "REWARD_INVALID_TRANSITION", "Награда уже обработана или переход статуса запрещён")
		return
	}
	eventBranch, branchErr := resolveEventBranch(r, tx, tenant, strings.TrimSpace(in.BranchID))
	if branchErr != nil {
		fail(w, 404, "BRANCH_NOT_FOUND", "Активный филиал не найден или недоступен сотруднику")
		return
	}
	var reservedAt, reservedTo, redeemedAt, cancelledAt any
	var reservedBy, redeemedBy, cancelledBy any
	if target == "reserved" {
		reservedAt = now
		reservedTo = now.Add(time.Duration(in.ReservationMinutes) * time.Minute)
		reservedBy = claims.Subject
	}
	if target == "redeemed" {
		redeemedAt = now
		redeemedBy = claims.Subject
	}
	if target == "cancelled" {
		cancelledAt = now
		cancelledBy = claims.Subject
	}
	_, err = tx.Exec(r.Context(), `UPDATE customer_rewards SET status=$3,reserved_at=coalesce($4,reserved_at),reserved_until=$5,reserved_by=coalesce($6,reserved_by),redeemed_at=$7,redeemed_by=$8,cancelled_at=$9,cancelled_by=$10 WHERE company_id=$1 AND id=$2`, tenant, r.PathValue("id"), target, reservedAt, reservedTo, reservedBy, redeemedAt, redeemedBy, cancelledAt, cancelledBy)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO reward_transactions(company_id,reward_id,customer_id,actor_id,operation,from_status,to_status,reason,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$5,$7,nullif($8,'')) ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, tenant, r.PathValue("id"), customerID, claims.Subject, target, status, strings.TrimSpace(in.Reason), idempotency)
	}
	if err == nil && target == "redeemed" {
		err = appendCustomerEvent(r, tx, tenant, customerID, "reward.redeemed", eventBranch, "reward-redeemed:"+r.PathValue("id"), map[string]any{"rewardId": r.PathValue("id"), "reason": strings.TrimSpace(in.Reason)})
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "REWARD_TRANSITION_FAILED", "Не удалось изменить статус награды")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": r.PathValue("id"), "status": target, "reservedUntil": reservedTo}})
}
func (a *api) reserveReward(w http.ResponseWriter, r *http.Request) {
	a.transitionReward(w, r, "reserved")
}
func (a *api) redeemReward(w http.ResponseWriter, r *http.Request) {
	a.transitionReward(w, r, "redeemed")
}
func (a *api) cancelReward(w http.ResponseWriter, r *http.Request) {
	a.transitionReward(w, r, "cancelled")
}
func (a *api) updateReward(w http.ResponseWriter, r *http.Request) {
	var in rewardStatusInput
	if !decode(w, r, &in) {
		return
	}
	r.Body = http.NoBody
	switch in.Status {
	case "reserved":
		a.transitionRewardWithInput(w, r, "reserved", in)
	case "redeemed":
		a.transitionRewardWithInput(w, r, "redeemed", in)
	case "cancelled":
		a.transitionRewardWithInput(w, r, "cancelled", in)
	default:
		fail(w, 422, "VALIDATION_ERROR", "Доступны статусы reserved, redeemed или cancelled")
	}
}
func (a *api) transitionRewardWithInput(w http.ResponseWriter, r *http.Request, target string, in rewardStatusInput) {
	r.Header.Set("X-Reward-Reason", in.Reason)
	a.transitionRewardDirect(w, r, target, in)
}

func (a *api) transitionRewardDirect(w http.ResponseWriter, r *http.Request, target string, in rewardStatusInput) {
	// Compatibility endpoint delegates through the same locked state machine.
	if in.ReservationMinutes < 1 {
		in.ReservationMinutes = 15
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать операцию")
		return
	}
	defer tx.Rollback(r.Context())
	tenant := companyID(r)
	claims := identity(r)
	idempotency := strings.TrimSpace(in.IdempotencyKey)
	if idempotency != "" {
		idempotency = target + ":" + r.PathValue("id") + ":" + idempotency
		var existing string
		if tx.QueryRow(r.Context(), `SELECT reward_id FROM reward_transactions WHERE company_id=$1 AND idempotency_key=$2`, tenant, idempotency).Scan(&existing) == nil && existing == r.PathValue("id") {
			write(w, 200, envelope{Success: true, Data: map[string]any{"id": existing, "status": target, "idempotentReplay": true}})
			return
		}
	}
	var customerID, status string
	var expires, reserved *time.Time
	if tx.QueryRow(r.Context(), `SELECT customer_id,status,expires_at,reserved_until FROM customer_rewards WHERE company_id=$1 AND id=$2 FOR UPDATE`, tenant, r.PathValue("id")).Scan(&customerID, &status, &expires, &reserved) != nil {
		fail(w, 404, "REWARD_NOT_FOUND", "Награда не найдена")
		return
	}
	now := time.Now()
	status = effectiveRewardStatus(status, reserved, now)
	valid := (target == "reserved" && status == "available") || (target == "redeemed" && (status == "available" || (status == "reserved" && (reserved == nil || reserved.After(now))))) || (target == "cancelled" && (status == "available" || status == "reserved"))
	if expires != nil && !expires.After(now) {
		valid = false
	}
	if !valid {
		fail(w, 409, "REWARD_INVALID_TRANSITION", "Награда недоступна или уже обработана")
		return
	}
	eventBranch, branchErr := resolveEventBranch(r, tx, tenant, strings.TrimSpace(in.BranchID))
	if branchErr != nil {
		fail(w, 404, "BRANCH_NOT_FOUND", "Активный филиал не найден или недоступен сотруднику")
		return
	}
	reservedUntil := any(nil)
	if target == "reserved" {
		reservedUntil = now.Add(time.Duration(in.ReservationMinutes) * time.Minute)
	}
	_, err = tx.Exec(r.Context(), `UPDATE customer_rewards SET status=$3,reserved_at=CASE WHEN $3='reserved' THEN now() ELSE reserved_at END,reserved_until=$4,reserved_by=CASE WHEN $3='reserved' THEN $5 ELSE reserved_by END,redeemed_at=CASE WHEN $3='redeemed' THEN now() ELSE NULL END,redeemed_by=CASE WHEN $3='redeemed' THEN $5 ELSE NULL END,cancelled_at=CASE WHEN $3='cancelled' THEN now() ELSE NULL END,cancelled_by=CASE WHEN $3='cancelled' THEN $5 ELSE NULL END WHERE company_id=$1 AND id=$2`, tenant, r.PathValue("id"), target, reservedUntil, claims.Subject)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO reward_transactions(company_id,reward_id,customer_id,actor_id,operation,from_status,to_status,reason,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$5,$7,nullif($8,'')) ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, tenant, r.PathValue("id"), customerID, claims.Subject, target, status, in.Reason, idempotency)
	}
	if err == nil && target == "redeemed" {
		err = appendCustomerEvent(r, tx, tenant, customerID, "reward.redeemed", eventBranch, "reward-redeemed:"+r.PathValue("id"), map[string]any{"rewardId": r.PathValue("id"), "reason": strings.TrimSpace(in.Reason)})
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "REWARD_TRANSITION_FAILED", "Не удалось изменить статус награды")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"status": target, "reservedUntil": reservedUntil}})
}

// expireRewards sweeps this company on demand. StartRewardExpiryWorker runs the
// same sweep on a timer for every company, so staff never have to remember to
// press anything; this endpoint stays for the staff member who wants a reward
// released right now rather than within the next few minutes.
func (a *api) expireRewards(w http.ResponseWriter, r *http.Request) {
	released, expired, err := sweepCompanyRewardExpiry(r.Context(), a.db, companyID(r), identity(r).Subject)
	if err != nil {
		fail(w, 500, "REWARD_EXPIRY_FAILED", "Не удалось обработать истёкшие награды")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]int{"expired": expired, "reservationsReleased": released}})
}

func (a *api) rewardTransactions(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT rt.id,rt.operation,rt.from_status,rt.to_status,rt.reason,rt.created_at,coalesce(u.first_name,'System') FROM reward_transactions rt LEFT JOIN users u ON u.id=rt.actor_id WHERE rt.company_id=$1 AND rt.reward_id=$2 ORDER BY rt.created_at`, companyID(r), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить аудит награды")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, op, to, reason, actor string
		var from *string
		var created time.Time
		if rows.Scan(&id, &op, &from, &to, &reason, &created, &actor) == nil {
			items = append(items, map[string]any{"id": id, "operation": op, "fromStatus": from, "toStatus": to, "reason": reason, "actor": actor, "createdAt": created})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

// effectiveRewardStatus resolves a reservation that has run out.
//
// The reward list already presents such a reward as available, and
// POST /rewards/expire releases it back to available — but that endpoint is
// only ever called by hand. Until someone called it, staff saw an available
// reward in the interface and were refused when they tried to hand it over,
// with a message saying it had already been processed.
func effectiveRewardStatus(status string, reservedUntil *time.Time, now time.Time) string {
	if status == "reserved" && reservedUntil != nil && !reservedUntil.After(now) {
		return "available"
	}
	return status
}

func rewardCycle(mode string, now time.Time) string {
	switch mode {
	case "calendar_month":
		return now.Format("2006-01")
	case "calendar_year":
		return now.Format("2006")
	default:
		return "lifetime"
	}
}
func (a *api) evaluateRewardEvent(r *http.Request, tx pgx.Tx, tenant, customerID, event string, currentValue int) ([]string, error) {
	rows, err := tx.Query(r.Context(), `SELECT rr.id,rr.definition_id,rr.threshold,rr.progress_mode FROM reward_rules rr JOIN reward_definitions rd ON rd.id=rr.definition_id WHERE rr.company_id=$1 AND rr.event_type=$2 AND rr.is_active AND rd.is_active AND rd.deleted_at IS NULL ORDER BY rr.priority`, tenant, event)
	if err != nil {
		return nil, err
	}
	type rule struct {
		id, definition, mode string
		threshold            int
	}
	rules := []rule{}
	for rows.Next() {
		var x rule
		if rows.Scan(&x.id, &x.definition, &x.threshold, &x.mode) == nil {
			rules = append(rules, x)
		}
	}
	rows.Close()
	issued := []string{}
	for _, x := range rules {
		cycle := rewardCycle(x.mode, time.Now())
		value := currentValue
		if x.mode == "repeat" {
			value = currentValue % x.threshold
			if value == 0 {
				value = x.threshold
			}
		}
		status := "in_progress"
		if value >= x.threshold {
			status = "available"
		}
		var progressID string
		err = tx.QueryRow(r.Context(), `INSERT INTO customer_reward_progress(company_id,customer_id,rule_id,cycle_key,current_value,target_value,status,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7::varchar,CASE WHEN $7::varchar='available' THEN now() END) ON CONFLICT(customer_id,rule_id,cycle_key) DO UPDATE SET current_value=excluded.current_value,target_value=excluded.target_value,status=CASE WHEN customer_reward_progress.status='completed' THEN 'completed' ELSE excluded.status END,completed_at=CASE WHEN excluded.status='available' THEN coalesce(customer_reward_progress.completed_at,now()) ELSE customer_reward_progress.completed_at END,updated_at=now() RETURNING id`, tenant, customerID, x.id, cycle, value, x.threshold, status).Scan(&progressID)
		if err != nil {
			return issued, err
		}
		if status == "in_progress" && x.threshold-value == 1 {
			key := fmt.Sprintf("reward-almost:%s:%s:%s:%d", x.id, customerID, cycle, currentValue)
			err = appendCustomerEvent(r, tx, tenant, customerID, "reward.almost_unlocked", "", key, map[string]any{"ruleId": x.id, "current": value, "target": x.threshold, "remaining": 1})
			if err != nil {
				return issued, err
			}
		}
		if status == "available" {
			key := "rule:" + x.id + ":" + customerID + ":" + cycle
			if x.mode == "repeat" {
				key = fmt.Sprintf("rule:%s:%s:%d", x.id, customerID, currentValue/x.threshold)
			}
			rewardID, e := a.issueRewardTx(r, tx, tenant, customerID, x.definition, x.id, &progressID, "Автоматически по правилу", key)
			if e == nil {
				issued = append(issued, rewardID)
				_, _ = tx.Exec(r.Context(), `UPDATE customer_reward_progress SET status='completed',completed_at=now() WHERE id=$1`, progressID)
			} else if e.Error() != "repeat" && e.Error() != "cooldown" && e.Error() != "inventory" {
				return issued, e
			}
		}
	}
	return issued, nil
}
