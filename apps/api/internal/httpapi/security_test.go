package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cookies))
	}
	byName := map[string]*http.Cookie{}
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Fatalf("cookie %s is missing security attributes", cookie.Name)
		}
	}
	if byName["tappix_guest_access"].Path != "/api/v1" {
		t.Fatal("guest access cookie has unexpected path")
	}
	if byName["tappix_guest_refresh"].Path != "/api/v1/auth" || byName["tappix_guest_refresh"].MaxAge != 90*24*60*60 {
		t.Fatal("guest refresh cookie has unexpected scope or lifetime")
	}
}
