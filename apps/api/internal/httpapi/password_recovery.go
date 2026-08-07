package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type forgotPasswordInput struct {
	Email string `json:"email"`
}

type resetPasswordInput struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

func (a *api) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var in forgotPasswordInput
	if !decode(w, r, &in) {
		return
	}
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if !strings.Contains(email, "@") || strings.ContainsAny(email, "\r\n") {
		fail(w, 422, "VALIDATION_ERROR", "Укажите корректный email")
		return
	}
	var userID string
	if a.db.QueryRow(r.Context(), `SELECT id FROM users WHERE lower(email)=$1 AND status='active' AND deleted_at IS NULL`, email).Scan(&userID) == nil {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err == nil {
			token := base64.RawURLEncoding.EncodeToString(buf)
			_ = a.redis.Set(r.Context(), "password-reset:"+tokenHash(token), userID, 30*time.Minute).Err()
			link := strings.TrimRight(envValue("APP_URL", "http://localhost:8088"), "/") + "/login?reset=" + token
			from := envValue("SMTP_FROM", "Tappix <noreply@tappix.kz>")
			message := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Tappix password reset\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nСсылка для смены пароля действует 30 минут:\n%s", from, email, link))
			_ = smtp.SendMail(envValue("SMTP_HOST", "mailpit")+":"+envValue("SMTP_PORT", "1025"), nil, fromAddress(from), []string{email}, message)
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]bool{"accepted": true}})
}

func (a *api) resetPassword(w http.ResponseWriter, r *http.Request) {
	var in resetPasswordInput
	if !decode(w, r, &in) {
		return
	}
	if len(in.NewPassword) < 8 {
		fail(w, 422, "WEAK_PASSWORD", "Пароль должен содержать минимум 8 символов")
		return
	}
	key := "password-reset:" + tokenHash(in.Token)
	userID, err := a.redis.Get(r.Context(), key).Result()
	if err != nil {
		fail(w, 410, "RESET_TOKEN_EXPIRED", "Ссылка недействительна или срок её действия истёк")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "PASSWORD_ERROR", "Не удалось изменить пароль")
		return
	}
	if _, err = a.db.Exec(r.Context(), `UPDATE users SET password_hash=$2,updated_at=now() WHERE id=$1`, userID, string(hash)); err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось изменить пароль")
		return
	}
	_ = a.redis.Del(r.Context(), key).Err()
	a.revokeUserSessions(r, userID)
	write(w, 200, envelope{Success: true, Data: map[string]bool{"passwordChanged": true}})
}

func (a *api) listSessions(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	ids, _ := a.redis.SMembers(r.Context(), "sessions:"+claims.Subject).Result()
	items := []map[string]any{}
	for _, id := range ids {
		raw, err := a.redis.Get(r.Context(), "sessionmeta:"+id).Bytes()
		if err != nil {
			_ = a.redis.SRem(r.Context(), "sessions:"+claims.Subject, id).Err()
			continue
		}
		var item map[string]any
		if json.Unmarshal(raw, &item) == nil {
			items = append(items, item)
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) deleteSession(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	id := r.PathValue("id")
	if !a.redis.SIsMember(r.Context(), "sessions:"+claims.Subject, id).Val() {
		fail(w, 404, "SESSION_NOT_FOUND", "Сессия не найдена")
		return
	}
	_ = a.redis.Del(r.Context(), "refresh:"+id, "sessionmeta:"+id).Err()
	_ = a.redis.SRem(r.Context(), "sessions:"+claims.Subject, id).Err()
	write(w, 200, envelope{Success: true, Data: map[string]bool{"revoked": true}})
}

func (a *api) revokeUserSessions(r *http.Request, userID string) {
	ids, _ := a.redis.SMembers(r.Context(), "sessions:"+userID).Result()
	keys := []string{"sessions:" + userID}
	for _, id := range ids {
		keys = append(keys, "refresh:"+id, "sessionmeta:"+id)
	}
	_ = a.redis.Del(r.Context(), keys...).Err()
}
