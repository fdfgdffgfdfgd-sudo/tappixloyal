package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

type approvalDecisionInput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func (a *api) requestBonusApproval(w http.ResponseWriter, r *http.Request, in bonusInput) {
	tenant := companyID(r)
	claims := identity(r)
	operation := "bonus." + in.Operation
	var id, status string
	err := a.db.QueryRow(r.Context(), `INSERT INTO operation_approvals(company_id,customer_id,requested_by,branch_id,operation,amount,reason,idempotency_key)
		SELECT $1,c.id,$2,b.id,$3,$4,$5,coalesce(nullif($6,''),gen_random_uuid()::text)
		FROM customers c JOIN branches b ON b.company_id=c.company_id AND b.id=$7 AND b.is_active AND b.deleted_at IS NULL
		WHERE c.company_id=$1 AND c.id=$8 AND c.deleted_at IS NULL AND b.id=(SELECT branch_id FROM users WHERE company_id=$1 AND id=$2)
		ON CONFLICT(company_id,idempotency_key) DO UPDATE SET idempotency_key=operation_approvals.idempotency_key RETURNING id,status`, tenant, claims.Subject, operation, in.Amount, strings.TrimSpace(in.Description), strings.TrimSpace(in.IdempotencyKey), strings.TrimSpace(in.BranchID), r.PathValue("id")).Scan(&id, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "CUSTOMER_OR_BRANCH_NOT_FOUND", "Клиент или филиал не найден либо недоступен сотруднику")
		return
	}
	if err != nil {
		fail(w, 500, "APPROVAL_REQUEST_FAILED", "Не удалось отправить заявку владельцу")
		return
	}
	a.recordRisk(r, r.PathValue("id"), in.BranchID, operation, "blocked", "Крупная операция отправлена владельцу на подтверждение", map[string]any{"amount": in.Amount, "approvalId": id})
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data) VALUES($1,$2,'loyalty.approval.requested','operation_approval',$3,$4,jsonb_build_object('operation',$5::text,'amount',$6::integer,'customerId',$7::text,'branchId',$8::text))`, tenant, claims.Subject, id, r.Header.Get("X-Request-ID"), operation, in.Amount, r.PathValue("id"), in.BranchID)
	write(w, 202, envelope{Success: true, Data: map[string]any{"id": id, "status": status, "approvalRequired": true, "message": "Заявка отправлена владельцу"}})
}

func (a *api) listOperationApprovals(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "pending"
	}
	if status != "pending" && status != "approved" && status != "rejected" && status != "expired" && status != "all" {
		fail(w, 422, "VALIDATION_ERROR", "Неизвестный статус заявки")
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT a.id,a.operation,a.amount,a.reason,a.status,a.requested_at,a.expires_at,coalesce(a.decided_at,a.requested_at),coalesce(a.decision_reason,''),c.id,c.first_name,c.last_name,b.id,b.name,u.first_name,u.last_name
		FROM operation_approvals a JOIN customers c ON c.id=a.customer_id AND c.company_id=a.company_id JOIN branches b ON b.id=a.branch_id AND b.company_id=a.company_id JOIN users u ON u.id=a.requested_by AND u.company_id=a.company_id
		WHERE a.company_id=$1 AND ($2='all' OR a.status=$2) ORDER BY CASE WHEN a.status='pending' THEN 0 ELSE 1 END,a.requested_at DESC LIMIT 200`, companyID(r), status)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить заявки")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, operation, reason, approvalStatus, decisionReason, customerID, firstName, lastName, branchID, branchName, requesterFirst, requesterLast string
		var amount int
		var requestedAt, expiresAt, decidedAt time.Time
		if rows.Scan(&id, &operation, &amount, &reason, &approvalStatus, &requestedAt, &expiresAt, &decidedAt, &decisionReason, &customerID, &firstName, &lastName, &branchID, &branchName, &requesterFirst, &requesterLast) == nil {
			items = append(items, map[string]any{"id": id, "operation": operation, "amount": amount, "reason": reason, "status": approvalStatus, "requestedAt": requestedAt, "expiresAt": expiresAt, "decidedAt": decidedAt, "decisionReason": decisionReason, "customerId": customerID, "customer": strings.TrimSpace(firstName + " " + lastName), "branchId": branchID, "branch": branchName, "requester": strings.TrimSpace(requesterFirst + " " + requesterLast)})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) decideOperationApproval(w http.ResponseWriter, r *http.Request) {
	var in approvalDecisionInput
	if !decode(w, r, &in) {
		return
	}
	in.Decision = strings.TrimSpace(in.Decision)
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Decision != "approved" && in.Decision != "rejected" {
		fail(w, 422, "VALIDATION_ERROR", "Выберите одобрение или отклонение")
		return
	}
	if len([]rune(in.Reason)) < 4 {
		fail(w, 422, "REASON_REQUIRED", "Укажите причину решения — минимум 4 символа")
		return
	}
	tenant := companyID(r)
	claims := identity(r)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать рассмотрение")
		return
	}
	defer tx.Rollback(r.Context())
	var customerID, requesterID, branchID, operation, reason, status string
	var amount int
	var expiresAt time.Time
	err = tx.QueryRow(r.Context(), `SELECT customer_id,requested_by,branch_id,operation,amount,reason,status,expires_at FROM operation_approvals WHERE company_id=$1 AND id=$2 FOR UPDATE`, tenant, r.PathValue("id")).Scan(&customerID, &requesterID, &branchID, &operation, &amount, &reason, &status, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "APPROVAL_NOT_FOUND", "Заявка не найдена")
		return
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить заявку")
		return
	}
	if status != "pending" {
		fail(w, 409, "APPROVAL_ALREADY_DECIDED", "Заявка уже рассмотрена")
		return
	}
	if !expiresAt.After(time.Now()) {
		_, _ = tx.Exec(r.Context(), `UPDATE operation_approvals SET status='expired',decided_at=now(),decided_by=$3,decision_reason='Истёк срок рассмотрения' WHERE company_id=$1 AND id=$2`, tenant, r.PathValue("id"), claims.Subject)
		_ = tx.Commit(r.Context())
		fail(w, 409, "APPROVAL_EXPIRED", "Срок рассмотрения заявки истёк")
		return
	}
	if in.Decision == "rejected" {
		_, err = tx.Exec(r.Context(), `UPDATE operation_approvals SET status='rejected',decided_at=now(),decided_by=$3,decision_reason=$4 WHERE company_id=$1 AND id=$2`, tenant, r.PathValue("id"), claims.Subject, in.Reason)
	} else {
		var balance int
		err = tx.QueryRow(r.Context(), `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, tenant, customerID).Scan(&balance)
		next := balance + amount
		ledgerOperation := "credit"
		eventType := "bonus.earned"
		if operation == "bonus.debit" {
			next = balance - amount
			ledgerOperation = "debit"
			eventType = "bonus.spent"
		}
		if err == nil && next < 0 {
			fail(w, 409, "INSUFFICIENT_POINTS", "Баланс клиента изменился — бонусов недостаточно")
			return
		}
		var ledgerID string
		if err == nil {
			err = tx.QueryRow(r.Context(), `INSERT INTO bonus_ledger(company_id,customer_id,created_by,operation,amount,balance_after,description,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, tenant, customerID, requesterID, ledgerOperation, amount, next, reason, "approval:"+r.PathValue("id")).Scan(&ledgerID)
		}
		if err == nil && ledgerOperation == "credit" {
			err = posintegration.IssueBonusLot(r.Context(), tx, tenant, customerID, ledgerID, "", amount)
		}
		if err == nil && ledgerOperation == "debit" {
			err = posintegration.ConsumeBonusLots(r.Context(), tx, tenant, customerID, ledgerID, "", amount)
		}
		if err == nil {
			_, err = tx.Exec(r.Context(), `UPDATE customers SET total_points=$3,updated_at=now() WHERE company_id=$1 AND id=$2`, tenant, customerID, next)
		}
		if err == nil {
			err = appendCustomerEvent(r, tx, tenant, customerID, eventType, branchID, "approval-bonus:"+r.PathValue("id"), map[string]any{"amount": amount, "balanceAfter": next, "reason": reason, "manual": true, "approved": true, "approvalId": r.PathValue("id")})
		}
		if err == nil {
			_, err = tx.Exec(r.Context(), `UPDATE operation_approvals SET status='approved',decided_at=now(),decided_by=$3,decision_reason=$4,executed_at=now(),bonus_ledger_id=$5 WHERE company_id=$1 AND id=$2`, tenant, r.PathValue("id"), claims.Subject, in.Reason, ledgerID)
		}
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data) VALUES($1,$2,$3,'operation_approval',$4,$5,jsonb_build_object('decision',$6::text,'reason',$7::text,'customerId',$8::text,'amount',$9::integer))`, tenant, claims.Subject, "loyalty.approval."+in.Decision, r.PathValue("id"), r.Header.Get("X-Request-ID"), in.Decision, in.Reason, customerID, amount)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "APPROVAL_DECISION_FAILED", "Не удалось сохранить решение")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": r.PathValue("id"), "status": in.Decision, "executed": in.Decision == "approved"}})
}
