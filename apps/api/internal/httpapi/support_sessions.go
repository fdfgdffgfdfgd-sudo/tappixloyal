package httpapi

import (
	"net/http"
	"strings"
	"time"
)

func (a *api) startSupportSession(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CompanyID string `json:"companyId"`
		Reason    string `json:"reason"`
		Minutes   int    `json:"minutes"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.CompanyID == "" || len(in.Reason) < 10 {
		fail(w, 400, "VALIDATION_ERROR", "Укажите компанию и причину минимум из 10 символов")
		return
	}
	if in.Minutes < 1 {
		in.Minutes = 30
	}
	if in.Minutes > 60 {
		in.Minutes = 60
	}
	claims := identity(r)
	expires := time.Now().Add(time.Duration(in.Minutes) * time.Minute)
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO support_sessions(company_id,actor_id,reason,expires_at) SELECT id,$2,$3,$4 FROM companies WHERE id=$1 AND status='active' AND deleted_at IS NULL RETURNING id`, in.CompanyID, claims.Subject, in.Reason, expires).Scan(&id)
	if err != nil {
		fail(w, 404, "COMPANY_NOT_FOUND", "Компания не найдена или недоступна")
		return
	}
	token, err := a.signJWT(tokenClaims{Subject: claims.Subject, CompanyID: in.CompanyID, Role: "company_owner", Audience: "support", SupportSessionID: id, ExpiresAt: expires.Unix()})
	if err != nil {
		fail(w, 500, "TOKEN_ERROR", "Не удалось открыть сессию поддержки")
		return
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,after_data) VALUES($1,$2,'support.session.started','support_session',$3,jsonb_build_object('reason',$4,'expiresAt',$5))`, in.CompanyID, claims.Subject, id, in.Reason, expires)
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "companyId": in.CompanyID, "expiresAt": expires, "accessToken": token, "banner": "Вы работаете в режиме поддержки. Все действия записываются."}})
}

func (a *api) listSupportSessions(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT s.id,s.company_id,c.name,s.reason,s.expires_at,s.revoked_at,s.created_at FROM support_sessions s JOIN companies c ON c.id=s.company_id ORDER BY s.created_at DESC LIMIT 100`)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить сессии поддержки")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, cid, name, reason string
		var expires, created time.Time
		var revoked *time.Time
		if err := rows.Scan(&id, &cid, &name, &reason, &expires, &revoked, &created); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить сессии поддержки")
			return
		}
		items = append(items, map[string]any{"id": id, "companyId": cid, "company": name, "reason": reason, "expiresAt": expires, "revokedAt": revoked, "createdAt": created, "active": revoked == nil && expires.After(time.Now())})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить сессии поддержки")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) revokeSupportSession(w http.ResponseWriter, r *http.Request) {
	claims := identity(r)
	id := r.PathValue("id")
	tag, err := a.db.Exec(r.Context(), `UPDATE support_sessions SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, id)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "SUPPORT_SESSION_NOT_FOUND", "Сессия не найдена")
		return
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,entity_type,entity_id) VALUES($1,'support.session.revoked','support_session',$2)`, claims.Subject, id)
	write(w, 200, envelope{Success: true, Data: map[string]bool{"revoked": true}})
}
