package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFProtectionForCookieAuthenticatedMutation(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := csrfProtection(next)

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/customer/me", nil)
	request.AddCookie(&http.Cookie{Name: "tappix_guest_access", Value: "session"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("mutation without CSRF token returned %d, want 403", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodPatch, "/api/v1/customer/me", nil)
	request.AddCookie(&http.Cookie{Name: "tappix_guest_access", Value: "session"})
	request.AddCookie(&http.Cookie{Name: "tappix_guest_csrf", Value: "safe-token"})
	request.Header.Set("X-CSRF-Token", "safe-token")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("mutation with matching CSRF token returned %d, want 204", recorder.Code)
	}
}

func TestSessionCookiePrefixSeparatesAudiences(t *testing.T) {
	tests := map[string]string{
		"owner":       "tappix",
		"staff":       "tappix",
		"super_admin": "tappix_platform",
		"platform":    "tappix_platform",
		"customer":    "tappix_guest",
		"guest":       "tappix_guest",
	}
	for role, want := range tests {
		if got := sessionCookiePrefix(role); got != want {
			t.Fatalf("sessionCookiePrefix(%q) = %q, want %q", role, got, want)
		}
	}
}

func TestGuestSessionCookiesAreHttpOnlyAndScoped(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	recorder := httptest.NewRecorder()
	(&api{}).setSessionCookies(recorder, "access", "refresh", "customer")
	cookies := recorder.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("got %d cookies, want 3", len(cookies))
	}
	byName := map[string]*http.Cookie{}
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
		if !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || (cookie.Name != "tappix_guest_csrf" && !cookie.HttpOnly) {
			t.Fatalf("cookie %s is missing security attributes", cookie.Name)
		}
	}
	if byName["tappix_guest_access"].Path != "/" {
		t.Fatal("guest access cookie has unexpected path")
	}
	if byName["tappix_guest_refresh"].Path != "/api/v1/auth" || byName["tappix_guest_refresh"].MaxAge != 90*24*60*60 {
		t.Fatal("guest refresh cookie has unexpected scope or lifetime")
	}
	if byName["tappix_guest_csrf"].HttpOnly || byName["tappix_guest_csrf"].Path != "/" {
		t.Fatal("guest CSRF cookie must be readable by the frontend and available on all routes")
	}
}
