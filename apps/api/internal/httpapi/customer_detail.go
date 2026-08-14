package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

type customerUpdateInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Birthday  string `json:"birthday"`
	Level     string `json:"level"`
}
type bonusInput struct {
	Operation   string `json:"operation"`
	Amount      int    `json:"amount"`
	Description string `json:"description"`
}

func (a *api) updateCustomer(w http.ResponseWriter, r *http.Request) {
	var in customerUpdateInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.FirstName) == "" || len(strings.TrimSpace(in.Phone)) < 7 {
		fail(w, 422, "VALIDATION_ERROR", "Имя и телефон обязательны")
		return
	}
	if in.Email != "" && (!strings.Contains(in.Email, "@") || strings.ContainsAny(in.Email, "\r\n")) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите корректный Email")
		return
	}
	var birthday any
	if in.Birthday != "" {
		birthday = in.Birthday
	}
	if in.Level == "" {
		in.Level = "basic"
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE customers SET first_name=$3,last_name=$4,phone=$5,email=nullif($6,''),birthday=$7,level=$8,updated_at=now() WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), r.PathValue("id"), strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Email), birthday, in.Level)
	if err != nil {
		if strings.Contains(err.Error(), "customers_company_phone_unique") {
			fail(w, 409, "PHONE_EXISTS", "Этот телефон уже используется")
			return
		}
		fail(w, 500, "DATABASE_ERROR", "Не удалось обновить клиента")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Клиент не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"updated": true}})
}
func (a *api) deleteCustomer(w http.ResponseWriter, r *http.Request) {
	tag, err := a.db.Exec(r.Context(), `UPDATE customers SET deleted_at=now(),updated_at=now() WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Клиент не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"archived": true}})
}
func (a *api) customerAdminHistory(w http.ResponseWriter, r *http.Request) {
	tenant := companyID(r)
	customerID := r.PathValue("id")
	bonusRows, err := a.db.Query(r.Context(), `SELECT id,operation,amount,balance_after,description,created_at FROM bonus_ledger WHERE company_id=$1 AND customer_id=$2 ORDER BY created_at DESC LIMIT 100`, tenant, customerID)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить историю")
		return
	}
	bonuses := []map[string]any{}
	for bonusRows.Next() {
		var id, operation, description string
		var amount, balance int
		var created time.Time
		if bonusRows.Scan(&id, &operation, &amount, &balance, &description, &created) == nil {
			bonuses = append(bonuses, map[string]any{"id": id, "operation": operation, "amount": amount, "balanceAfter": balance, "description": description, "createdAt": created})
		}
	}
	bonusRows.Close()
	visitRows, err := a.db.Query(r.Context(), `SELECT v.id,v.points_added,v.comment,v.created_at,b.name,coalesce(u.first_name,'System') FROM visits v JOIN branches b ON b.id=v.branch_id LEFT JOIN users u ON u.id=v.employee_id WHERE v.company_id=$1 AND v.customer_id=$2 ORDER BY v.created_at DESC LIMIT 100`, tenant, customerID)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить посещения")
		return
	}
	defer visitRows.Close()
	visits := []map[string]any{}
	for visitRows.Next() {
		var id, comment, branch, employee string
		var points int
		var created time.Time
		if visitRows.Scan(&id, &points, &comment, &created, &branch, &employee) == nil {
			visits = append(visits, map[string]any{"id": id, "pointsAdded": points, "comment": comment, "createdAt": created, "branch": branch, "employee": employee})
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"bonuses": bonuses, "visits": visits}})
}
func (a *api) customerBonus(w http.ResponseWriter, r *http.Request) {
	var in bonusInput
	if !decode(w, r, &in) {
		return
	}
	if (in.Operation != "credit" && in.Operation != "debit") || in.Amount <= 0 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите операцию и положительную сумму")
		return
	}
	if len([]rune(strings.TrimSpace(in.Description))) < 4 {
		fail(w, 422, "REASON_REQUIRED", "Укажите причину ручной операции — минимум 4 символа")
		return
	}
	tenant := companyID(r)
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	if claims.Role == "employee" && ((in.Operation == "debit" && in.Amount > 5000) || (in.Operation == "credit" && in.Amount > 10000)) {
		a.recordRisk(r, r.PathValue("id"), "", "bonus."+in.Operation, "blocked", "Сумма требует подтверждения владельца", map[string]any{"amount": in.Amount})
		fail(w, 403, "MANAGER_APPROVAL_REQUIRED", "Для этой суммы нужно подтверждение владельца")
		return
	}
	var recent bool
	_ = a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM bonus_ledger WHERE company_id=$1 AND customer_id=$2 AND created_by=$3 AND operation=$4 AND amount=$5 AND created_at>now()-interval '30 seconds')`, tenant, r.PathValue("id"), claims.Subject, in.Operation, in.Amount).Scan(&recent)
	if recent {
		a.recordRisk(r, r.PathValue("id"), "", "bonus."+in.Operation, "blocked", "Повтор ручной бонусной операции", map[string]any{"amount": in.Amount, "windowSeconds": 30})
		fail(w, 409, "DUPLICATE_BONUS_OPERATION", "Такая операция уже выполнена. Проверьте баланс перед повтором")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать операцию")
		return
	}
	defer tx.Rollback(r.Context())
	var balance int
	err = tx.QueryRow(r.Context(), `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, tenant, r.PathValue("id")).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Клиент не найден")
		return
	}
	next := balance + in.Amount
	if in.Operation == "debit" {
		next = balance - in.Amount
	}
	if next < 0 {
		fail(w, 409, "INSUFFICIENT_POINTS", "Недостаточно бонусов для списания")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE customers SET total_points=$3,updated_at=now() WHERE company_id=$1 AND id=$2`, tenant, r.PathValue("id"), next)
	if err == nil {
		var ledgerID string
		err = tx.QueryRow(r.Context(), `INSERT INTO bonus_ledger(company_id,customer_id,created_by,operation,amount,balance_after,description) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, tenant, r.PathValue("id"), claims.Subject, in.Operation, in.Amount, next, in.Description).Scan(&ledgerID)
		if err == nil && in.Operation == "credit" {
			err = posintegration.IssueBonusLot(r.Context(), tx, tenant, r.PathValue("id"), ledgerID, "", in.Amount)
		}
		if err == nil && in.Operation == "debit" {
			err = posintegration.ConsumeBonusLots(r.Context(), tx, tenant, r.PathValue("id"), ledgerID, "", in.Amount)
		}
		if err == nil {
			eventType := "bonus.earned"
			if in.Operation == "debit" {
				eventType = "bonus.spent"
			}
			err = appendCustomerEvent(r, tx, tenant, r.PathValue("id"), eventType, "", "bonus:"+ledgerID, map[string]any{"amount": in.Amount, "balanceAfter": next, "reason": in.Description, "manual": true})
		}
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "BONUS_FAILED", "Не удалось выполнить бонусную операцию")
		return
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data) VALUES($1,$2,'loyalty.manual_bonus','customer',$3,$4,jsonb_build_object('operation',$5::text,'amount',$6::integer,'reason',$7::text,'balanceAfter',$8::integer))`, tenant, claims.Subject, r.PathValue("id"), r.Header.Get("X-Request-ID"), in.Operation, in.Amount, in.Description, next)
	write(w, 201, envelope{Success: true, Data: map[string]int{"balance": next, "amount": in.Amount}})
}
func (a *api) listVisits(w http.ResponseWriter, r *http.Request) {
	customerID := r.URL.Query().Get("customerId")
	rows, err := a.db.Query(r.Context(), `SELECT v.id,v.customer_id,c.first_name,c.last_name,b.name,v.points_added,v.comment,v.created_at FROM visits v JOIN customers c ON c.id=v.customer_id JOIN branches b ON b.id=v.branch_id WHERE v.company_id=$1 AND ($2='' OR v.customer_id::text=$2) ORDER BY v.created_at DESC LIMIT 100`, companyID(r), customerID)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить посещения")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, cid, first, last, branch, comment string
		var points int
		var created time.Time
		if rows.Scan(&id, &cid, &first, &last, &branch, &points, &comment, &created) == nil {
			items = append(items, map[string]any{"id": id, "customerId": cid, "customer": strings.TrimSpace(first + " " + last), "branch": branch, "pointsAdded": points, "comment": comment, "createdAt": created})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
