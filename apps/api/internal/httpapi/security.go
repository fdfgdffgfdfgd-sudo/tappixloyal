package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func (a *api) setSessionCookies(w http.ResponseWriter, access, refresh, role string) {
	secure := os.Getenv("COOKIE_SECURE") == "true"
	prefix := sessionCookiePrefix(role)
	refreshMaxAge := 30 * 24 * 60 * 60
	if role == "customer" {
		refreshMaxAge = 90 * 24 * 60 * 60
	}
	http.SetCookie(w, &http.Cookie{Name: prefix + "_access", Value: access, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: 15 * 60})
	http.SetCookie(w, &http.Cookie{Name: prefix + "_refresh", Value: refresh, Path: "/api/v1/auth", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: refreshMaxAge})
	csrf := make([]byte, 32)
	_, _ = rand.Read(csrf)
	http.SetCookie(w, &http.Cookie{Name: prefix + "_csrf", Value: base64.RawURLEncoding.EncodeToString(csrf), Path: "/", Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: refreshMaxAge})
}

func sessionCookiePrefix(role string) string {
	switch role {
	case "super_admin", "platform":
		return "tappix_platform"
	case "customer", "guest":
		return "tappix_guest"
	default:
		return "tappix"
	}
}

func (a *api) clearSessionCookies(w http.ResponseWriter, audience string) {
	prefix := sessionCookiePrefix(audience)
	for _, item := range []struct{ name, path string }{{prefix + "_access", "/"}, {prefix + "_access", "/api/v1"}, {prefix + "_refresh", "/api/v1/auth"}, {prefix + "_csrf", "/"}} {
		http.SetCookie(w, &http.Cookie{Name: item.name, Value: "", Path: item.path, HttpOnly: true, MaxAge: -1, SameSite: http.SameSiteStrictMode})
	}
}

func csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.Header.Get("Authorization") != "" || csrfExemptPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		audience := "business"
		if strings.HasPrefix(r.URL.Path, "/api/v1/customer/") || r.URL.Query().Get("aud") == "guest" {
			audience = "guest"
		} else if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") || r.URL.Query().Get("aud") == "platform" {
			audience = "platform"
		}
		prefix := sessionCookiePrefix(audience)
		_, accessErr := r.Cookie(prefix + "_access")
		_, refreshErr := r.Cookie(prefix + "_refresh")
		if accessErr != nil && refreshErr != nil {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(prefix + "_csrf")
		provided := r.Header.Get("X-CSRF-Token")
		if err != nil || provided == "" || !hmac.Equal([]byte(cookie.Value), []byte(provided)) {
			fail(w, http.StatusForbidden, "CSRF_INVALID", "Защитный токен недействителен. Обновите страницу")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func csrfExemptPath(path string) bool {
	switch path {
	case "/api/v1/auth/login", "/api/v1/auth/forgot-password", "/api/v1/auth/reset-password", "/api/v1/customer/register", "/api/v1/customer/login", "/api/v1/customer/otp/request", "/api/v1/customer/otp/verify":
		return true
	}
	return strings.HasPrefix(path, "/api/v1/public/") || strings.HasPrefix(path, "/api/v1/integrations/")
}

func (a *api) createMFAChallenge(r *http.Request, userID string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := base64.RawURLEncoding.EncodeToString(b)
	_ = a.redis.Set(r.Context(), "mfa:challenge:"+token, userID, 5*time.Minute).Err()
	return token
}

func (a *api) consumeMFAChallenge(r *http.Request, challenge, userID string) bool {
	key := "mfa:challenge:" + challenge
	value, err := a.redis.GetDel(r.Context(), key).Result()
	return err == nil && hmac.Equal([]byte(value), []byte(userID))
}

func generateTOTPSecret() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

func validTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	for drift := int64(-1); drift <= 1; drift++ {
		if hmac.Equal([]byte(totpCode(secret, now.Unix()/30+drift)), []byte(code)) {
			return true
		}
	}
	return false
}

func totpCode(secret string, counter int64) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 15
	n := (uint32(sum[offset])&127)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", n%1000000)
}

func (a *api) mfaSetup(w http.ResponseWriter, r *http.Request) {
	claims := identity(r)
	secret := generateTOTPSecret()
	if _, err := a.db.Exec(r.Context(), `UPDATE users SET mfa_secret=$2,mfa_enabled=false WHERE id=$1`, claims.Subject, secret); err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось подготовить MFA")
		return
	}
	var email string
	_ = a.db.QueryRow(r.Context(), `SELECT email FROM users WHERE id=$1`, claims.Subject).Scan(&email)
	uri := "otpauth://totp/Tappix:" + email + "?secret=" + secret + "&issuer=Tappix&algorithm=SHA1&digits=6&period=30"
	write(w, 200, envelope{Success: true, Data: map[string]string{"secret": secret, "otpauthUri": uri}})
}

func (a *api) mfaEnable(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	claims := identity(r)
	var secret *string
	if a.db.QueryRow(r.Context(), `SELECT mfa_secret FROM users WHERE id=$1`, claims.Subject).Scan(&secret) != nil || secret == nil || !validTOTP(*secret, in.Code, time.Now()) {
		fail(w, 400, "INVALID_MFA_CODE", "Неверный код подтверждения")
		return
	}
	_, _ = a.db.Exec(r.Context(), `UPDATE users SET mfa_enabled=true,mfa_enabled_at=now() WHERE id=$1`, claims.Subject)
	write(w, 200, envelope{Success: true, Data: map[string]bool{"enabled": true}})
}

func (a *api) mfaDisable(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	claims := identity(r)
	var secret *string
	if a.db.QueryRow(r.Context(), `SELECT mfa_secret FROM users WHERE id=$1 AND mfa_enabled`, claims.Subject).Scan(&secret) != nil || secret == nil || !validTOTP(*secret, in.Code, time.Now()) {
		fail(w, 400, "INVALID_MFA_CODE", "Неверный код подтверждения")
		return
	}
	_, _ = a.db.Exec(r.Context(), `UPDATE users SET mfa_enabled=false,mfa_secret=NULL,mfa_enabled_at=NULL WHERE id=$1`, claims.Subject)
	write(w, 200, envelope{Success: true, Data: map[string]bool{"enabled": false}})
}

func identity(r *http.Request) tokenClaims {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	return claims
}

func (a *api) requirePermission(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := identity(r)
		if claims.Role == "super_admin" {
			next.ServeHTTP(w, r)
			return
		}
		if claims.Role == "customer" {
			fail(w, 403, "PERMISSION_DENIED", "Недостаточно прав")
			return
		}
		var membershipRole string
		err := a.db.QueryRow(r.Context(), `SELECT role FROM company_memberships WHERE company_id=$1 AND user_id=$2 AND status='active'`, claims.CompanyID, claims.Subject).Scan(&membershipRole)
		if err != nil {
			fail(w, 403, "MEMBERSHIP_REQUIRED", "Нет активного доступа к компании")
			return
		}
		var allowed bool
		_ = a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM role_permissions WHERE role=$1 AND permission=$2)`, membershipRole, permission).Scan(&allowed)
		if !allowed {
			fail(w, 403, "PERMISSION_DENIED", "Недостаточно прав для этой операции")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseMinutes(value string, fallback, maximum int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	if n > maximum {
		return maximum
	}
	return n
}
