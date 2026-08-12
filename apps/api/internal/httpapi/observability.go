package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type responseTelemetry struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseTelemetry) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseTelemetry) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += n
	return n, err
}

var requestMetrics sync.Map
var activeRequests atomic.Int64

func metricCounter(key string) *atomic.Uint64 {
	value, _ := requestMetrics.LoadOrStore(key, &atomic.Uint64{})
	return value.(*atomic.Uint64)
}

func (a *api) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		activeRequests.Add(1)
		defer activeRequests.Add(-1)
		response := &responseTelemetry{ResponseWriter: w}
		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		statusClass := strconv.Itoa(response.status/100) + "xx"
		metricCounter(r.Method + "|" + route + "|" + statusClass).Add(1)
		claims := identity(r)
		slog.Info("http.request", "event_type", "http.request", "method", r.Method, "route", route, "status", response.status, "duration_ms", time.Since(started).Milliseconds(), "response_bytes", response.bytes, "request_id", response.Header().Get("X-Request-ID"), "tenant_id", claims.CompanyID, "actor_id", claims.Subject)
	})
}

func (a *api) metrics(w http.ResponseWriter, r *http.Request) {
	configured := strings.TrimSpace(os.Getenv("METRICS_TOKEN"))
	if configured != "" && r.Header.Get("Authorization") != "Bearer "+configured {
		fail(w, http.StatusUnauthorized, "METRICS_UNAUTHORIZED", "Требуется токен мониторинга")
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprint(w, "# HELP tappix_http_requests_total Total HTTP requests by route and status class.\n# TYPE tappix_http_requests_total counter\n")
	keys := []string{}
	requestMetrics.Range(func(key, _ any) bool { keys = append(keys, key.(string)); return true })
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.Split(key, "|")
		value, _ := requestMetrics.Load(key)
		fmt.Fprintf(w, "tappix_http_requests_total{method=%q,route=%q,status_class=%q} %d\n", parts[0], parts[1], parts[2], value.(*atomic.Uint64).Load())
	}
	fmt.Fprintf(w, "# HELP tappix_http_requests_active Current active HTTP requests.\n# TYPE tappix_http_requests_active gauge\ntappix_http_requests_active %d\n", activeRequests.Load())
}

func logDomainEvent(r *http.Request, eventType, customerID string, attributes ...any) {
	claims := identity(r)
	fields := []any{"event_type", eventType, "tenant_id", claims.CompanyID, "actor_id", claims.Subject, "request_id", r.Header.Get("X-Request-ID")}
	if customerID != "" {
		fields = append(fields, "customer_id", customerID)
	}
	fields = append(fields, attributes...)
	slog.Info(eventType, fields...)
}
