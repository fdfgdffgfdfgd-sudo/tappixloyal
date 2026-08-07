package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"time"
)

func (a *api) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		limit := 120
		window := time.Minute
		if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/customer/login" || r.URL.Path == "/api/v1/customer/otp/request" || r.URL.Path == "/api/v1/customer/otp/verify" {
			limit = 10
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
