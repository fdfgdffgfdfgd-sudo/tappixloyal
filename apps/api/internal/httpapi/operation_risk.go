package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (a *api) recordRisk(r *http.Request, customerID, branchID, operation, severity, reason string, metadata map[string]any) {
	raw, _ := json.Marshal(metadata)
	claims := identity(r)
	_, _ = a.db.Exec(r.Context(), `INSERT INTO operation_risk_flags(company_id,customer_id,actor_id,branch_id,operation,severity,reason,rule_code,metadata)
		VALUES($1,nullif($2,'')::uuid,nullif($3,'')::uuid,nullif($4,'')::uuid,$5,$6,$7,$8,$9::jsonb)`, companyID(r), customerID, claims.Subject, branchID, operation, severity, reason, riskRuleCode(operation, reason), raw)
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data)
		VALUES($1,nullif($2,'')::uuid,'security.operation_blocked','customer',nullif($3,'')::uuid,$4,jsonb_build_object('operation',$5::text,'reason',$6::text,'severity',$7::text))`, companyID(r), claims.Subject, customerID, r.Header.Get("X-Request-ID"), operation, reason, severity)
}

func riskRuleCode(operation, reason string) string {
	lower := strings.ToLower(reason)
	if strings.Contains(lower, "повтор") || strings.Contains(lower, "duplicate") {
		return "duplicate_operation"
	}
	if operation == "visit.create" {
		return "rapid_visit"
	}
	if strings.HasPrefix(operation, "bonus.") {
		return "large_manual_adjustment"
	}
	return "behavior_anomaly"
}

func (a *api) riskInvestigations(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	severity := strings.TrimSpace(r.URL.Query().Get("severity"))
	if status == "" {
		status = "open"
	}
	if status != "all" && status != "open" && status != "reviewed" && status != "dismissed" {
		fail(w, 422, "VALIDATION_ERROR", "Неизвестный статус расследования")
		return
	}
	if severity != "" && severity != "warning" && severity != "blocked" {
		fail(w, 422, "VALIDATION_ERROR", "Неизвестная критичность")
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT f.id,f.operation,f.rule_code,f.severity,f.status,f.reason,f.metadata,f.created_at,coalesce(f.customer_id::text,''),
		coalesce(c.first_name||' '||c.last_name,''),coalesce(c.phone,''),coalesce(b.name,''),coalesce(u.first_name||' '||u.last_name,''),coalesce(rv.first_name||' '||rv.last_name,''),coalesce(f.resolution,'')
		FROM operation_risk_flags f LEFT JOIN customers c ON c.id=f.customer_id AND c.company_id=f.company_id
		LEFT JOIN branches b ON b.id=f.branch_id AND b.company_id=f.company_id LEFT JOIN users u ON u.id=f.actor_id
		LEFT JOIN users rv ON rv.id=f.reviewed_by WHERE f.company_id=$1 AND ($2='all' OR f.status=$2) AND ($3='' OR f.severity=$3)
		ORDER BY CASE f.severity WHEN 'blocked' THEN 0 ELSE 1 END,f.created_at DESC LIMIT 250`, companyID(r), status, severity)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить расследования")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, operation, rule, severityValue, statusValue, reason, customerID, customer, phone, branch, actor, reviewer, resolution string
		var metadata json.RawMessage
		var created time.Time
		if rows.Scan(&id, &operation, &rule, &severityValue, &statusValue, &reason, &metadata, &created, &customerID, &customer, &phone, &branch, &actor, &reviewer, &resolution) == nil {
			items = append(items, map[string]any{"id": id, "operation": operation, "ruleCode": rule, "severity": severityValue, "status": statusValue, "reason": reason, "metadata": metadata, "createdAt": created, "customerId": customerID, "customer": strings.TrimSpace(customer), "phone": phone, "branch": branch, "actor": strings.TrimSpace(actor), "reviewer": strings.TrimSpace(reviewer), "resolution": resolution})
		}
	}
	var open, blocked, reviewed int
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FILTER(WHERE status='open'),count(*) FILTER(WHERE status='open' AND severity='blocked'),count(*) FILTER(WHERE status='reviewed' AND reviewed_at>=now()-interval '30 days') FROM operation_risk_flags WHERE company_id=$1`, companyID(r)).Scan(&open, &blocked, &reviewed)
	write(w, 200, envelope{Success: true, Data: map[string]any{"items": items, "summary": map[string]int{"open": open, "blocked": blocked, "reviewed30Days": reviewed}}})
}

type riskDecisionInput struct {
	Decision   string `json:"decision"`
	Resolution string `json:"resolution"`
}

func (a *api) decideRiskInvestigation(w http.ResponseWriter, r *http.Request) {
	var in riskDecisionInput
	if !decode(w, r, &in) {
		return
	}
	in.Decision = strings.TrimSpace(in.Decision)
	in.Resolution = strings.TrimSpace(in.Resolution)
	if in.Decision != "reviewed" && in.Decision != "dismissed" {
		fail(w, 422, "VALIDATION_ERROR", "Выберите результат расследования")
		return
	}
	if len([]rune(in.Resolution)) < 6 {
		fail(w, 422, "RESOLUTION_REQUIRED", "Опишите результат расследования — минимум 6 символов")
		return
	}
	claims := identity(r)
	tag, err := a.db.Exec(r.Context(), `UPDATE operation_risk_flags SET status=$3,resolution=$4,reviewed_at=now(),reviewed_by=$5,updated_at=now() WHERE company_id=$1 AND id=$2 AND status='open'`, companyID(r), r.PathValue("id"), in.Decision, in.Resolution, claims.Subject)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось завершить расследование")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "INVESTIGATION_ALREADY_CLOSED", "Расследование уже закрыто или не найдено")
		return
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data) VALUES($1,$2,'security.investigation.closed','operation_risk_flag',$3,$4,jsonb_build_object('decision',$5::text,'resolution',$6::text))`, companyID(r), claims.Subject, r.PathValue("id"), r.Header.Get("X-Request-ID"), in.Decision, in.Resolution)
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": r.PathValue("id"), "status": in.Decision}})
}

func (a *api) customerRisk(w http.ResponseWriter, r *http.Request) {
	var exists bool
	if err := a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL)`, companyID(r), r.PathValue("id")).Scan(&exists); err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось проверить клиента")
		return
	}
	if !exists {
		fail(w, 404, "CUSTOMER_NOT_FOUND", "Клиент не найден")
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT f.id,f.operation,f.severity,f.status,f.reason,f.created_at,coalesce(b.name,''),coalesce(u.first_name,'System')
		FROM operation_risk_flags f LEFT JOIN branches b ON b.id=f.branch_id AND b.company_id=f.company_id LEFT JOIN users u ON u.id=f.actor_id
		WHERE f.company_id=$1 AND f.customer_id=$2 ORDER BY f.created_at DESC LIMIT 50`, companyID(r), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить контроль операций")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, operation, severity, status, reason, branch, actor string
		var created any
		if rows.Scan(&id, &operation, &severity, &status, &reason, &created, &branch, &actor) == nil {
			items = append(items, map[string]any{"id": id, "operation": operation, "severity": severity, "status": status, "reason": reason, "createdAt": created, "branch": branch, "actor": actor})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
