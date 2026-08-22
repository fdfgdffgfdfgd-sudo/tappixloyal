package httpapi

import (
	"context"
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

type telemetryIdentity struct {
	tenantID string
	actorID  string
}

type telemetryIdentityKey struct{}

func attachTelemetryIdentity(r *http.Request) (*http.Request, *telemetryIdentity) {
	value := &telemetryIdentity{}
	return r.WithContext(context.WithValue(r.Context(), telemetryIdentityKey{}, value)), value
}

func captureTelemetryIdentity(r *http.Request, claims tokenClaims) {
	if value, ok := r.Context().Value(telemetryIdentityKey{}).(*telemetryIdentity); ok {
		value.tenantID = claims.CompanyID
		value.actorID = claims.Subject
	}
}

func telemetryRoute(r *http.Request) string {
	if r.Pattern != "" && r.Pattern != "/api/v1/" {
		return r.Pattern
	}
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for index, segment := range segments {
		if len(segment) > 24 || isUUID(segment) {
			segments[index] = "{id}"
		}
	}
	if len(segments) == 0 || segments[0] == "" {
		return "/"
	}
	return "/" + strings.Join(segments, "/")
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

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
var requestDurationMicros sync.Map
var activeRequests atomic.Int64
var workersReady atomic.Bool

// MarkWorkersReady exposes the embedded worker subsystem to health checks and
// monitoring without creating a second control plane.
func MarkWorkersReady() { workersReady.Store(true) }

func metricCounter(key string) *atomic.Uint64 {
	value, _ := requestMetrics.LoadOrStore(key, &atomic.Uint64{})
	return value.(*atomic.Uint64)
}

func (a *api) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r, telemetryIdentity := attachTelemetryIdentity(r)
		started := time.Now()
		activeRequests.Add(1)
		defer activeRequests.Add(-1)
		response := &responseTelemetry{ResponseWriter: w}
		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		route := telemetryRoute(r)
		statusClass := strconv.Itoa(response.status/100) + "xx"
		metricCounter(r.Method + "|" + route + "|" + statusClass).Add(1)
		duration := time.Since(started)
		durationCounter, _ := requestDurationMicros.LoadOrStore(r.Method+"|"+route, &atomic.Uint64{})
		durationCounter.(*atomic.Uint64).Add(uint64(duration.Microseconds()))
		slog.Info("http.request", "event_type", "http.request", "method", r.Method, "route", route, "status", response.status, "duration_ms", duration.Milliseconds(), "response_bytes", response.bytes, "request_id", response.Header().Get("X-Request-ID"), "tenant_id", telemetryIdentity.tenantID, "actor_id", telemetryIdentity.actorID)
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
	fmt.Fprint(w, "# HELP tappix_http_request_duration_seconds_sum Total request duration by route.\n# TYPE tappix_http_request_duration_seconds_sum counter\n")
	durationKeys := []string{}
	requestDurationMicros.Range(func(key, _ any) bool { durationKeys = append(durationKeys, key.(string)); return true })
	sort.Strings(durationKeys)
	for _, key := range durationKeys {
		parts := strings.Split(key, "|")
		value, _ := requestDurationMicros.Load(key)
		fmt.Fprintf(w, "tappix_http_request_duration_seconds_sum{method=%q,route=%q} %.6f\n", parts[0], parts[1], float64(value.(*atomic.Uint64).Load())/1_000_000)
	}
	fmt.Fprintf(w, "# HELP tappix_http_requests_active Current active HTTP requests.\n# TYPE tappix_http_requests_active gauge\ntappix_http_requests_active %d\n", activeRequests.Load())
	fmt.Fprintf(w, "# HELP tappix_workers_ready Whether embedded background workers completed startup.\n# TYPE tappix_workers_ready gauge\ntappix_workers_ready %d\n", boolMetric(workersReady.Load()))
	postgresUp := a.db.Ping(r.Context()) == nil
	redisUp := a.redis.Ping(r.Context()).Err() == nil
	fmt.Fprintf(w, "# HELP tappix_dependency_up Whether a required dependency is available.\n# TYPE tappix_dependency_up gauge\ntappix_dependency_up{dependency=%q} %d\ntappix_dependency_up{dependency=%q} %d\n", "postgres", boolMetric(postgresUp), "redis", boolMetric(redisUp))
	var failedJobs, queuedJobs int
	if err := a.db.QueryRow(r.Context(), `SELECT count(*) FILTER (WHERE status='failed' AND created_at>now()-interval '1 hour'), count(*) FILTER (WHERE status IN('pending','processing')) FROM integration_jobs`).Scan(&failedJobs, &queuedJobs); err == nil {
		fmt.Fprintf(w, "# HELP tappix_integration_jobs_failed_last_hour Failed integration jobs created in the last hour.\n# TYPE tappix_integration_jobs_failed_last_hour gauge\ntappix_integration_jobs_failed_last_hour %d\n", failedJobs)
		fmt.Fprintf(w, "# HELP tappix_integration_jobs_pending Pending integration jobs.\n# TYPE tappix_integration_jobs_pending gauge\ntappix_integration_jobs_pending %d\n", queuedJobs)
	}
}

func boolMetric(value bool) int {
	if value {
		return 1
	}
	return 0
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
