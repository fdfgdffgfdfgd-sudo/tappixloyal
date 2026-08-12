package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type loginInput struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	MFAChallenge string `json:"mfaChallenge"`
	MFACode      string `json:"mfaCode"`
}
type refreshInput struct {
	RefreshToken string `json:"refreshToken"`
}
type tokenClaims struct {
	Subject          string `json:"sub"`
	CompanyID        string `json:"companyId"`
	Role             string `json:"role"`
	Audience         string `json:"aud"`
	SupportSessionID string `json:"supportSessionId,omitempty"`
	ExpiresAt        int64  `json:"exp"`
}
type sessionData struct {
	UserID    string `json:"userId"`
	CompanyID string `json:"companyId"`
	Role      string `json:"role"`
	SessionID string `json:"sessionId"`
}
type authKey string

const identityKey authKey = "identity"

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var in loginInput
	if !decode(w, r, &in) {
		return
	}
	var userID, firstName, lastName, role, hash string
	var mfaEnabled bool
	var mfaSecret *string
	var nullableCompany *string
	err := a.db.QueryRow(r.Context(), `SELECT u.id,u.company_id,u.first_name,u.last_name,u.role,u.password_hash,u.mfa_enabled,u.mfa_secret FROM users u LEFT JOIN companies c ON c.id=u.company_id WHERE lower(u.email)=lower($1) AND u.status='active' AND u.deleted_at IS NULL AND (u.role='super_admin' OR c.status='active')`, strings.TrimSpace(in.Email)).Scan(&userID, &nullableCompany, &firstName, &lastName, &role, &hash, &mfaEnabled, &mfaSecret)
	if errors.Is(err, pgx.ErrNoRows) || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		fail(w, 401, "INVALID_CREDENTIALS", "Неверный email или пароль")
		return
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось выполнить вход")
		return
	}
	companyID := ""
	if nullableCompany != nil {
		companyID = *nullableCompany
	}
	if mfaEnabled {
		if in.MFAChallenge == "" || !a.consumeMFAChallenge(r, in.MFAChallenge, userID) || mfaSecret == nil || !validTOTP(*mfaSecret, in.MFACode, time.Now()) {
			challenge := a.createMFAChallenge(r, userID)
			write(w, 200, envelope{Success: true, Data: map[string]any{"mfaRequired": true, "mfaChallenge": challenge}})
			return
		}
	}
	access, refresh, err := a.issueTokens(r, userID, companyID, role)
	if err != nil {
		fail(w, 500, "TOKEN_ERROR", "Не удалось создать сессию")
		return
	}
	_, _ = a.db.Exec(r.Context(), `UPDATE users SET last_login_at=now() WHERE id=$1`, userID)
	a.setSessionCookies(w, access, refresh, role)
	write(w, 200, envelope{Success: true, Data: map[string]any{"accessToken": access, "refreshToken": refresh, "mfaRequired": false, "user": map[string]string{"id": userID, "companyId": companyID, "firstName": firstName, "lastName": lastName, "role": role}}})
}

func (a *api) refresh(w http.ResponseWriter, r *http.Request) {
	var in refreshInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.RefreshToken == "" {
		prefix := sessionCookiePrefix(r.URL.Query().Get("aud"))
		if c, err := r.Cookie(prefix + "_refresh"); err == nil {
			in.RefreshToken = c.Value
		}
	}
	if in.RefreshToken == "" {
		fail(w, 401, "INVALID_REFRESH_TOKEN", "Сессия истекла, войдите снова")
		return
	}
	key := "refresh:" + tokenHash(in.RefreshToken)
	raw, err := a.redis.Get(r.Context(), key).Bytes()
	if err != nil {
		fail(w, 401, "INVALID_REFRESH_TOKEN", "Сессия истекла, войдите снова")
		return
	}
	var session sessionData
	if json.Unmarshal(raw, &session) != nil {
		fail(w, 401, "INVALID_REFRESH_TOKEN", "Сессия повреждена")
		return
	}
	_ = a.redis.Del(r.Context(), key).Err()
	_ = a.redis.SRem(r.Context(), "sessions:"+session.UserID, tokenHash(in.RefreshToken)).Err()
	_ = a.redis.Del(r.Context(), "sessionmeta:"+tokenHash(in.RefreshToken)).Err()
	access, refresh, err := a.issueTokens(r, session.UserID, session.CompanyID, session.Role)
	if err != nil {
		fail(w, 500, "TOKEN_ERROR", "Не удалось обновить сессию")
		return
	}
	a.setSessionCookies(w, access, refresh, session.Role)
	if session.Role == "customer" {
		write(w, 200, envelope{Success: true, Data: map[string]bool{"authenticated": true}})
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]string{"accessToken": access, "refreshToken": refresh}})
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	var in refreshInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.RefreshToken == "" {
		prefix := sessionCookiePrefix(r.URL.Query().Get("aud"))
		if c, err := r.Cookie(prefix + "_refresh"); err == nil {
			in.RefreshToken = c.Value
		}
	}
	hash := tokenHash(in.RefreshToken)
	if raw, err := a.redis.Get(r.Context(), "refresh:"+hash).Bytes(); err == nil {
		var session sessionData
		if json.Unmarshal(raw, &session) == nil {
			_ = a.redis.SRem(r.Context(), "sessions:"+session.UserID, hash).Err()
		}
	}
	_ = a.redis.Del(r.Context(), "refresh:"+hash, "sessionmeta:"+hash).Err()
	a.clearSessionCookies(w, r.URL.Query().Get("aud"))
	write(w, 200, envelope{Success: true, Data: map[string]bool{"loggedOut": true}})
}

func (a *api) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := ""
		if strings.HasPrefix(header, "Bearer ") {
			token = strings.TrimPrefix(header, "Bearer ")
		} else if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") || r.URL.Path == "/api/v1/audit" {
			if c, err := r.Cookie("tappix_platform_access"); err == nil {
				token = c.Value
			}
		} else if strings.HasPrefix(r.URL.Path, "/api/v1/customer/") {
			if c, err := r.Cookie("tappix_guest_access"); err == nil {
				token = c.Value
			}
		} else if c, err := r.Cookie("tappix_access"); err == nil {
			token = c.Value
		}
		if token == "" {
			fail(w, 401, "AUTH_REQUIRED", "Войдите в систему")
			return
		}
		claims, err := a.verifyJWT(token)
		if err != nil {
			fail(w, 401, "INVALID_ACCESS_TOKEN", "Сессия недействительна")
			return
		}
		guestRoute := strings.HasPrefix(r.URL.Path, "/api/v1/customer/")
		adminRoute := strings.HasPrefix(r.URL.Path, "/api/v1/admin/") || r.URL.Path == "/api/v1/audit"
		if guestRoute && claims.Audience != "guest" {
			fail(w, 403, "INVALID_AUDIENCE", "Эта сессия не предназначена для карты клиента")
			return
		}
		if !guestRoute && claims.Audience == "guest" {
			fail(w, 403, "INVALID_AUDIENCE", "Клиентская сессия не даёт доступ к кабинету бизнеса")
			return
		}
		if adminRoute && claims.Audience != "platform" {
			fail(w, 403, "INVALID_AUDIENCE", "Требуется сессия платформы")
			return
		}
		if claims.Role != "super_admin" {
			var active bool
			if a.db.QueryRow(r.Context(), `SELECT status='active' FROM companies WHERE id=$1 AND deleted_at IS NULL`, claims.CompanyID).Scan(&active) != nil || !active {
				fail(w, 403, "COMPANY_BLOCKED", "Доступ компании приостановлен")
				return
			}
		}
		if claims.SupportSessionID != "" {
			var active bool
			_ = a.db.QueryRow(r.Context(), `SELECT revoked_at IS NULL AND expires_at>now() FROM support_sessions WHERE id=$1 AND actor_id=$2 AND company_id=$3`, claims.SupportSessionID, claims.Subject, claims.CompanyID).Scan(&active)
			if !active {
				fail(w, 401, "SUPPORT_SESSION_EXPIRED", "Сессия поддержки завершена")
				return
			}
		}
		ctx := context.WithValue(r.Context(), identityKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (a *api) verifyJWT(token string) (tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return tokenClaims{}, errors.New("invalid token")
	}
	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, a.jwtSecret)
	_, _ = mac.Write([]byte(unsigned))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return tokenClaims{}, errors.New("invalid signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenClaims{}, err
	}
	var claims tokenClaims
	if json.Unmarshal(payload, &claims) != nil || claims.Subject == "" || (claims.CompanyID == "" && claims.Role != "super_admin") || claims.ExpiresAt < time.Now().Unix() {
		return tokenClaims{}, errors.New("expired token")
	}
	return claims, nil
}
func (a *api) me(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	write(w, 200, envelope{Success: true, Data: claims})
}
func companyID(r *http.Request) string {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	return claims.CompanyID
}
func (a *api) requireRoles(next http.Handler, roles ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := r.Context().Value(identityKey).(tokenClaims)
		for _, role := range roles {
			if claims.Role == role {
				next.ServeHTTP(w, r)
				return
			}
		}
		fail(w, 403, "FORBIDDEN", "Недостаточно прав для этой операции")
	})
}
func (a *api) requireModule(code string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := r.Context().Value(identityKey).(tokenClaims)
		if claims.Role == "super_admin" {
			next.ServeHTTP(w, r)
			return
		}
		if !a.moduleIncluded(r.Context(), claims.CompanyID, code) {
			fail(w, 403, "PLAN_UPGRADE_REQUIRED", "Функция недоступна на текущем тарифе")
			return
		}
		var enabled bool
		err := a.db.QueryRow(r.Context(), `SELECT m.is_core OR coalesce(cm.enabled,false) FROM modules m LEFT JOIN company_modules cm ON cm.module_code=m.code AND cm.company_id=$1 WHERE m.code=$2`, claims.CompanyID, code).Scan(&enabled)
		if err != nil || !enabled {
			fail(w, 403, "MODULE_DISABLED", "Модуль не подключён к подписке")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *api) issueTokens(r *http.Request, userID, companyID, role string) (string, string, error) {
	audience := "business"
	if role == "super_admin" {
		audience = "platform"
	} else if role == "customer" {
		audience = "guest"
	}
	claims := tokenClaims{Subject: userID, CompanyID: companyID, Role: role, Audience: audience, ExpiresAt: time.Now().Add(15 * time.Minute).Unix()}
	access, err := a.signJWT(claims)
	if err != nil {
		return "", "", err
	}
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	refresh := base64.RawURLEncoding.EncodeToString(buf)
	hash := tokenHash(refresh)
	raw, _ := json.Marshal(sessionData{UserID: userID, CompanyID: companyID, Role: role, SessionID: hash})
	meta, _ := json.Marshal(map[string]any{"id": hash, "createdAt": time.Now().UTC(), "userAgent": r.UserAgent(), "ip": r.RemoteAddr})
	pipe := a.redis.TxPipeline()
	sessionTTL := 30 * 24 * time.Hour
	if role == "customer" {
		sessionTTL = 90 * 24 * time.Hour
	}
	pipe.Set(r.Context(), "refresh:"+hash, raw, sessionTTL)
	pipe.Set(r.Context(), "sessionmeta:"+hash, meta, sessionTTL)
	pipe.SAdd(r.Context(), "sessions:"+userID, hash)
	pipe.Expire(r.Context(), "sessions:"+userID, sessionTTL)
	_, err = pipe.Exec(r.Context())
	return access, refresh, err
}
func (a *api) signJWT(claims tokenClaims) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadRaw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadRaw)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, a.jwtSecret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
