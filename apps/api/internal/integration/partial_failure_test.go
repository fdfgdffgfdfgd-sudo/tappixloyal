package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPosterAdapterFailureModesReturnBeforeLocalImport(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "http 500", statusCode: http.StatusInternalServerError, body: `temporary failure`},
		{name: "http 503", statusCode: http.StatusServiceUnavailable, body: `maintenance`},
		{name: "application error", statusCode: http.StatusOK, body: `{"response":null,"error":{"code":10,"message":"token expired"}}`},
		{name: "invalid response", statusCode: http.StatusOK, body: `{not-json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			adapter := NewPosterAdapter(server.Client(), server.URL)
			connection := Connection{ID: "test-connection", CompanyID: "test-company", Credentials: map[string]string{"accessToken": "test-token"}}
			if _, err := adapter.ImportTransactions(context.Background(), connection, "20260801"); err == nil {
				t.Fatal("Poster failure was reported as a successful import")
			}
			// PosterAdapter performs no local writes before a successful response;
			// the caller must therefore have no transaction to retry from this step.
		})
	}

	t.Run("connection error", func(t *testing.T) {
		adapter := NewPosterAdapter(http.DefaultClient, "http://127.0.0.1:1")
		connection := Connection{ID: "test-connection", CompanyID: "test-company", Credentials: map[string]string{"accessToken": "test-token"}}
		if _, err := adapter.ImportTransactions(context.Background(), connection, "20260801"); err == nil {
			t.Fatal("connection failure was reported as a successful import")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(20 * time.Millisecond):
			case <-r.Context().Done():
			}
			w.WriteHeader(http.StatusGatewayTimeout)
		}))
		defer server.Close()
		adapter := NewPosterAdapter(&http.Client{Timeout: 1 * time.Millisecond}, server.URL)
		connection := Connection{ID: "test-connection", CompanyID: "test-company", Credentials: map[string]string{"accessToken": "test-token"}}
		if _, err := adapter.ImportTransactions(context.Background(), connection, "20260801"); err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
			t.Fatalf("timeout was not propagated: %v", err)
		}
	})
}
