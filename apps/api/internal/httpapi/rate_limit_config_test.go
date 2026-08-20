package httpapi

import (
	"testing"
	"time"
)

func TestConfiguredRateKeepsProductionDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOGIN_RATE_LIMIT", "9999")
	t.Setenv("LOGIN_RATE_WINDOW", "1h")
	limit, window := configuredRate("LOGIN_RATE_LIMIT", "LOGIN_RATE_WINDOW", 10, time.Minute)
	if limit != 10 || window != time.Minute { t.Fatalf("production override applied: %d/%s", limit, window) }
}

func TestConfiguredRateAcceptsDevelopmentFiniteOverride(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("PUBLIC_RATE_LIMIT", "2000")
	t.Setenv("PUBLIC_RATE_WINDOW", "2m")
	limit, window := configuredRate("PUBLIC_RATE_LIMIT", "PUBLIC_RATE_WINDOW", 120, time.Minute)
	if limit != 2000 || window != 2*time.Minute { t.Fatalf("unexpected override: %d/%s", limit, window) }
}

func TestConfiguredRateRejectsMalformedAndNonPositiveValues(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	for _, value := range []string{"", "-1", "0", "not-a-number"} {
		t.Setenv("AUTH_RATE_LIMIT", value)
		t.Setenv("AUTH_RATE_WINDOW", value)
		limit, window := configuredRate("AUTH_RATE_LIMIT", "AUTH_RATE_WINDOW", 120, time.Minute)
		if limit != 120 || window != time.Minute { t.Fatalf("value %q bypassed safe defaults: %d/%s", value, limit, window) }
	}
}
