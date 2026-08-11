package integration

import (
	"context"
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
