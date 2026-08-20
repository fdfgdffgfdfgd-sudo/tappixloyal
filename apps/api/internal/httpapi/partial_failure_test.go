package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWhatsAppDeliveryFailureModesReturnError(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "http 500", statusCode: http.StatusInternalServerError, body: `provider error`},
		{name: "http 503", statusCode: http.StatusServiceUnavailable, body: `temporarily unavailable`},
		{name: "application error", statusCode: http.StatusOK, body: `{"error":{"message":"template rejected"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			t.Setenv("WHATSAPP_ACCESS_TOKEN", "local-test-token")
			t.Setenv("WHATSAPP_PHONE_NUMBER_ID", "local-phone-id")
			t.Setenv("WHATSAPP_API_BASE", server.URL)
			if _, err := sendWhatsAppText(context.Background(), "+77001234567", "test"); err == nil {
				t.Fatal("WhatsApp failure was reported as a successful delivery")
			}
		})
	}

	t.Run("connection error", func(t *testing.T) {
		t.Setenv("WHATSAPP_ACCESS_TOKEN", "local-test-token")
		t.Setenv("WHATSAPP_PHONE_NUMBER_ID", "local-phone-id")
		t.Setenv("WHATSAPP_API_BASE", "http://127.0.0.1:1")
		if _, err := sendWhatsAppText(context.Background(), "+77001234567", "test"); err == nil {
			t.Fatal("connection failure was reported as a successful delivery")
		}
	})

	t.Run("timeout context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()
		t.Setenv("WHATSAPP_ACCESS_TOKEN", "local-test-token")
		t.Setenv("WHATSAPP_PHONE_NUMBER_ID", "local-phone-id")
		t.Setenv("WHATSAPP_API_BASE", server.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 1)
		defer cancel()
		_, err := sendWhatsAppText(ctx, "+77001234567", "test")
		if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("timeout was not propagated: %v", err)
		}
	})

	t.Run("missing configuration does not call a provider", func(t *testing.T) {
		_ = os.Unsetenv("WHATSAPP_ACCESS_TOKEN")
		_ = os.Unsetenv("WHATSAPP_PHONE_NUMBER_ID")
		if _, err := sendWhatsAppText(context.Background(), "+77001234567", "test"); err == nil {
			t.Fatal("missing WhatsApp configuration was reported as success")
		}
	})
}
