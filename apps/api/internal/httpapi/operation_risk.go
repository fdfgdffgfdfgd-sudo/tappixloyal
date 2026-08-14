package httpapi

import (
	"encoding/json"
	"net/http"
)

func (a *api) recordRisk(r *http.Request, customerID, branchID, operation, severity, reason string, metadata map[string]any) {
	raw, _ := json.Marshal(metadata)
	claims := identity(r)
	_, _ = a.db.Exec(r.Context(), `INSERT INTO operation_risk_flags(company_id,customer_id,actor_id,branch_id,operation,severity,reason,metadata)
		VALUES($1,nullif($2,'')::uuid,nullif($3,'')::uuid,nullif($4,'')::uuid,$5,$6,$7,$8::jsonb)`, companyID(r), customerID, claims.Subject, branchID, operation, severity, reason, raw)
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data)
		VALUES($1,nullif($2,'')::uuid,'security.operation_blocked','customer',nullif($3,'')::uuid,$4,jsonb_build_object('operation',$5::text,'reason',$6::text,'severity',$7::text))`, companyID(r), claims.Subject, customerID, r.Header.Get("X-Request-ID"), operation, reason, severity)
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
