package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPosterAdapterReadOnlyImports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "poster-token" {
			t.Fatalf("access token was not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/spots.getSpots"):
			_, _ = w.Write([]byte(`{"response":[{"spot_id":"10","spot_name":"Абая"}]}`))
		case strings.HasSuffix(r.URL.Path, "/clients.getClients"):
			_, _ = w.Write([]byte(`{"response":[{"client_id":"20","firstname":"Алия","lastname":"К","phone":"8 700 123 45 67"}]}`))
		case strings.HasSuffix(r.URL.Path, "/transactions.getTransactions"), strings.HasSuffix(r.URL.Path, "/transactions.getTransaction"):
			_, _ = w.Write([]byte(`{"response":[{"transaction_id":"30","spot_id":"10","client_id":"20","date_close":"2026-08-08 12:00:00","sum":"150000","payed_sum":"140000"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	adapter := NewPosterAdapter(server.Client(), server.URL)
	connection := Connection{ID: "connection", CompanyID: "company", Credentials: map[string]string{"accessToken": "poster-token"}}

	locations, err := adapter.ListLocations(context.Background(), connection)
	if err != nil || len(locations) != 1 || locations[0].ExternalID != "10" {
		t.Fatalf("unexpected locations: %#v, %v", locations, err)
	}
	customers, err := adapter.ImportCustomers(context.Background(), connection, "")
	if err != nil || len(customers.Customers) != 1 || customers.Customers[0].Phone != "+77001234567" {
		t.Fatalf("unexpected customers: %#v, %v", customers, err)
	}
	transactions, err := adapter.ImportTransactions(context.Background(), connection, "20260801")
	if err != nil || len(transactions.Transactions) != 1 {
		t.Fatalf("unexpected transactions: %#v, %v", transactions, err)
	}
	transaction := transactions.Transactions[0]
	if transaction.ExternalID != "30" || transaction.GrossAmount != 1500 || transaction.NetAmount != 1400 || transaction.ExternalLocationID != "10" {
		t.Fatalf("unexpected canonical transaction: %#v", transaction)
	}
	single, err := adapter.GetTransaction(context.Background(), connection, "30")
	if err != nil || single.ExternalID != "30" || single.NetAmount != 1400 {
		t.Fatalf("unexpected Poster transaction lookup: %#v, %v", single, err)
	}
}

func TestPosterAdapterRejectsMissingToken(t *testing.T) {
	adapter := NewPosterAdapter(http.DefaultClient, "https://example.com")
	if err := adapter.Authorize(context.Background(), AuthorizationInput{}); err == nil {
		t.Fatal("missing token must be rejected")
	}
}

func TestPosterAdapterPaginationAndProviderFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("token") != "token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/clients.getClients":
			if r.URL.Query().Get("offset") != "40" {
				t.Fatalf("cursor was not forwarded: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"response":[`))
			for index := 0; index < 100; index++ {
				if index > 0 {
					_, _ = w.Write([]byte(","))
				}
				_, _ = fmt.Fprintf(w, `{"client_id":"%d","firstname":"Test","phone":"+77000000000"}`, index)
			}
			_, _ = w.Write([]byte(`]}`))
		case "/spots.getSpots":
			_, _ = w.Write([]byte(`{"response":null,"error":{"code":10,"message":"token expired"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	adapter := NewPosterAdapter(server.Client(), server.URL)
	connection := Connection{Credentials: map[string]string{"accessToken": "token"}}
	batch, err := adapter.ImportCustomers(context.Background(), connection, "40")
	if err != nil || len(batch.Customers) != 100 || batch.NextCursor != "140" {
		t.Fatalf("pagination failed: count=%d cursor=%q err=%v", len(batch.Customers), batch.NextCursor, err)
	}
	if _, err = adapter.ListLocations(context.Background(), connection); err == nil || !strings.Contains(err.Error(), "Poster API error") {
		t.Fatalf("provider error was not propagated: %v", err)
	}
}

func TestPosterCanonicalizesReturnsAndFallbackAmounts(t *testing.T) {
	transaction := canonicalPosterTransaction(Connection{ID: "connection", CompanyID: "company"}, map[string]any{"transaction_id": "55", "status": "refund", "sum": "250000", "payed_sum": "0", "date_start": "2026-08-10T10:00:00Z"})
	if transaction.Status != "cancelled" || transaction.NetAmount != 2500 || transaction.OccurredAt.IsZero() {
		t.Fatalf("return normalization failed: %#v", transaction)
	}
}
