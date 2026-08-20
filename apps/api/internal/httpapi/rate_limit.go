package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *api) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := clientIP(r)
		limit := 120
		window := time.Minute
		limitName, windowName := "PUBLIC_RATE_LIMIT", "PUBLIC_RATE_WINDOW"
		if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/customer/login" || r.URL.Path == "/api/v1/customer/otp/request" || r.URL.Path == "/api/v1/customer/otp/verify" {
			limitName, windowName = "LOGIN_RATE_LIMIT", "LOGIN_RATE_WINDOW"
			limit, window = configuredRate("LOGIN_RATE_LIMIT", "LOGIN_RATE_WINDOW", 10, time.Minute)
		} else if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") || strings.HasPrefix(r.URL.Path, "/api/v1/customer/") {
			limitName, windowName = "AUTH_RATE_LIMIT", "AUTH_RATE_WINDOW"
		}
		if limitName == "PUBLIC_RATE_LIMIT" || limitName == "AUTH_RATE_LIMIT" {
			limit, window = configuredRate(limitName, windowName, limit, window)
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/staff/customers/lookup") {
			limit = 20
		}
		bucket := time.Now().Unix() / int64(window.Seconds())
		key := "rate:" + host + ":" + r.URL.Path + ":" + strconv.FormatInt(bucket, 10)
		count, err := a.redis.Incr(r.Context(), key).Result()
		if err == nil && count == 1 {
			_ = a.redis.Expire(r.Context(), key, window).Err()
		}
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, limit-int(count))))
		if err == nil && count > int64(limit) {
			w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
			fail(w, 429, "RATE_LIMITED", "Слишком много запросов. Попробуйте позже")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func configuredPositiveInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(envOr(name, "")))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
func configuredPositiveDuration(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(envOr(name, ""))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// configuredRate accepts overrides only outside production. Invalid values and
// production deployments always use the safe defaults.
func configuredRate(limitName, windowName string, defaultLimit int, defaultWindow time.Duration) (int, time.Duration) {
	if strings.EqualFold(strings.TrimSpace(envOr("APP_ENV", "development")), "production") {
		return defaultLimit, defaultWindow
	}
	return configuredPositiveInt(limitName, defaultLimit), configuredPositiveDuration(windowName, defaultWindow)
}
