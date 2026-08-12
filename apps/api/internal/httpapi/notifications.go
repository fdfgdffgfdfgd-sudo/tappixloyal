package httpapi

import (
	"net/http"
	"os"
	"strings"
	"time"
)

type notificationInput struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

func (a *api) listNotifications(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id,channel,coalesce(recipient,''),coalesce(subject,''),body,status,sent_at,created_at FROM notifications WHERE company_id=$1 ORDER BY created_at DESC LIMIT 100`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить уведомления")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, channel, recipient, subject, body, status string
		var sent *time.Time
		var created time.Time
		if rows.Scan(&id, &channel, &recipient, &subject, &body, &status, &sent, &created) == nil {
			items = append(items, map[string]any{"id": id, "channel": channel, "recipient": recipient, "subject": subject, "body": body, "status": status, "sentAt": sent, "createdAt": created})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) sendNotification(w http.ResponseWriter, r *http.Request) {
	var in notificationInput
	if !decode(w, r, &in) {
		return
	}
	if !strings.Contains(in.Recipient, "@") || strings.ContainsAny(in.Recipient, "\r\n") || strings.TrimSpace(in.Subject) == "" || strings.ContainsAny(in.Subject, "\r\n") || strings.TrimSpace(in.Body) == "" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите email, тему и текст письма")
		return
	}
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO notifications(company_id,channel,recipient,subject,body,status) VALUES($1,'email',$2,$3,$4,'queued') RETURNING id`, companyID(r), strings.TrimSpace(in.Recipient), strings.TrimSpace(in.Subject), strings.TrimSpace(in.Body)).Scan(&id)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось создать уведомление")
		return
	}
	err = sendEmailWithTimeout(r.Context(), strings.TrimSpace(in.Recipient), strings.TrimSpace(in.Subject), strings.TrimSpace(in.Body))
	status := "sent"
	if err != nil {
		status = "failed"
	}
	if status == "sent" {
		_, _ = a.db.Exec(r.Context(), `UPDATE notifications SET status='sent',sent_at=now() WHERE id=$1`, id)
	} else {
		_, _ = a.db.Exec(r.Context(), `UPDATE notifications SET status='failed' WHERE id=$1`, id)
	}
	if err != nil {
		fail(w, 502, "EMAIL_SEND_FAILED", "SMTP не принял письмо")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "status": status}})
}
func envValue(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func fromAddress(value string) string {
	if start := strings.Index(value, "<"); start >= 0 {
		if end := strings.Index(value[start:], ">"); end > 0 {
			return value[start+1 : start+end]
		}
	}
	return value
}
