package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestObserveRecordsRouteWithoutPII(t *testing.T) {
	requestMetrics = syncMapForTest()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /customers/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := (&api{}).observe(mux)
	request := httptest.NewRequest(http.MethodGet, "/customers/secret-customer-id", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	found := false
	requestMetrics.Range(func(key, _ any) bool {
		value := key.(string)
		if strings.Contains(value, "secret-customer-id") {
			t.Fatalf("metric leaked path identifier: %s", value)
		}
		if strings.Contains(value, "GET /customers/{id}") {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("normalized route metric was not recorded")
	}
}

func syncMapForTest() sync.Map { return sync.Map{} }
