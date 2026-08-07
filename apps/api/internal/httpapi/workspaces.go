package httpapi

import "net/http"

func (a *api) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	rows, err := a.db.Query(r.Context(), `SELECT c.id,c.name,c.slug,m.role,coalesce(s.plan_code,'Starter') FROM company_memberships m JOIN companies c ON c.id=m.company_id AND c.status='active' AND c.deleted_at IS NULL LEFT JOIN LATERAL (SELECT plan_code FROM subscriptions WHERE company_id=c.id ORDER BY created_at DESC LIMIT 1) s ON true WHERE m.user_id=$1 AND m.status='active' ORDER BY c.name`, claims.Subject)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить рабочие пространства")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, slug, role, plan string
		if rows.Scan(&id, &name, &slug, &role, &plan) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "slug": slug, "role": role, "plan": plan, "current": id == claims.CompanyID})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) switchWorkspace(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	company := r.PathValue("id")
	var membershipRole string
	err := a.db.QueryRow(r.Context(), `SELECT m.role FROM company_memberships m JOIN companies c ON c.id=m.company_id WHERE m.user_id=$1 AND m.company_id=$2 AND m.status='active' AND c.status='active' AND c.deleted_at IS NULL`, claims.Subject, company).Scan(&membershipRole)
	if err != nil {
		fail(w, 403, "WORKSPACE_FORBIDDEN", "Нет доступа к этой компании")
		return
	}
	role := "employee"
	if membershipRole == "owner" || membershipRole == "admin" {
		role = "company_owner"
	}
	access, refresh, err := a.issueTokens(r, claims.Subject, company, role)
	if err != nil {
		fail(w, 500, "TOKEN_ERROR", "Не удалось переключить компанию")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]string{"accessToken": access, "refreshToken": refresh, "companyId": company, "role": role}})
}
