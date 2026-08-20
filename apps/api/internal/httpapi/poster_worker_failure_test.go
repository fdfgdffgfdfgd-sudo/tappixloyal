package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

func TestPosterWorkerProviderFailureUpdatesRetryState(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("POSTER_API_BASE_URL", server.URL)
	key := integrationEncryptionKey("worker-test-secret")
	credentials, err := encryptIntegrationSecret(key, []byte(`{"accessToken":"test-token"}`))
	if err != nil {
		t.Fatal(err)
	}
	var connectionID, jobID string
	if err = f.db.QueryRow(t.Context(), `INSERT INTO integration_connections(company_id,provider,name,status,encrypted_credentials) VALUES($1,'poster','QA Poster','active',$2) RETURNING id`, f.company, credentials).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(t.Context(), `INSERT INTO integration_location_mappings(company_id,connection_id,branch_id,external_location_id,status) VALUES($1,$2,$3,'spot-1','mapped')`, f.company, connectionID, f.branch); err != nil {
		t.Fatal(err)
	}
	var customerID string
	if err = f.db.QueryRow(t.Context(), `INSERT INTO customers(company_id,first_name,phone) VALUES($1,'Poster QA','+77001112233') RETURNING id`, f.company).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(t.Context(), `INSERT INTO integration_customer_links(company_id,connection_id,customer_id,external_customer_id,status,match_method) VALUES($1,$2,$3,'client-1','linked','external_id')`, f.company, connectionID, customerID); err != nil {
		t.Fatal(err)
	}
	if err = f.db.QueryRow(t.Context(), `INSERT INTO integration_jobs(company_id,connection_id,job_type,resource) VALUES($1,$2,'poster_transactions','transactions') RETURNING id`, f.company, connectionID).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.db.Exec(context.Background(), `DELETE FROM integration_jobs WHERE id=$1`, jobID)
		_, _ = f.db.Exec(context.Background(), `DELETE FROM integration_connections WHERE id=$1`, connectionID)
	})
	err = processNextPosterJob(t.Context(), f.db, key, server.Client())
	if err == nil || !strings.Contains(err.Error(), "Poster returned HTTP 503") {
		t.Fatalf("expected provider error, got %v", err)
	}
	var status string
	var attempts int
	var lastError string
	if err = f.db.QueryRow(t.Context(), `SELECT status,attempts,last_error FROM integration_jobs WHERE id=$1`, jobID).Scan(&status, &attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || attempts != 1 || !strings.Contains(lastError, "HTTP 503") {
		t.Fatalf("retry state not persisted: status=%s attempts=%d error=%q", status, attempts, lastError)
	}
}

func TestPosterWorkerPayloadDirectIngestComparison(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	_, _ = f.db.Exec(t.Context(), `DELETE FROM outbox_events WHERE idempotency_key IN ('outbox:poster:compare-first','outbox:poster:compare-second')`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":[{"transaction_id":"compare-first","date_close":"2026-08-19 10:00:00","spot_id":"spot-1","client_id":"client-1","sum":"1000","payed_sum":"1000"},{"transaction_id":"compare-second","date_close":"2026-08-19 10:01:00","spot_id":"spot-1","client_id":"client-1","sum":"-1000","payed_sum":"-1000"}]}`))
	}))
	defer server.Close()
	key := integrationEncryptionKey("worker-test-secret")
	credentials, err := encryptIntegrationSecret(key, []byte(`{"accessToken":"test-token"}`))
	if err != nil {
		t.Fatal(err)
	}
	var connectionID string
	if err = f.db.QueryRow(t.Context(), `INSERT INTO integration_connections(company_id,provider,name,status,encrypted_credentials) VALUES($1,'poster','Comparison','active',$2) RETURNING id`, f.company, credentials).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(t.Context(), `INSERT INTO integration_location_mappings(company_id,connection_id,branch_id,external_location_id,status) VALUES($1,$2,$3,'spot-1','mapped')`, f.company, connectionID, f.branch); err != nil {
		t.Fatal(err)
	}
	var customerID string
	if err = f.db.QueryRow(t.Context(), `INSERT INTO customers(company_id,first_name,phone) VALUES($1,'Comparison','+77002223344') RETURNING id`, f.company).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(t.Context(), `INSERT INTO integration_customer_links(company_id,connection_id,customer_id,external_customer_id,status,match_method) VALUES($1,$2,$3,'client-1','linked','external_id')`, f.company, connectionID, customerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.db.Exec(context.Background(), `DELETE FROM outbox_events WHERE company_id=$1`, f.company)
		_, _ = f.db.Exec(context.Background(), `DELETE FROM integration_connections WHERE id=$1`, connectionID)
	})
	adapter := posintegration.NewPosterAdapter(server.Client(), server.URL)
	connection := posintegration.Connection{ID: connectionID, CompanyID: f.company, Provider: "poster", Credentials: map[string]string{"accessToken": "test-token"}}
	batch, err := adapter.ImportTransactions(t.Context(), connection, "20260801")
	if err != nil {
		t.Fatal(err)
	}
	for index, tx := range batch.Transactions {
		t.Logf("worker-derived external_id=%q branch_id=%q external_location=%q customer_id=%q external_customer=%q campaign=%q original=%q occurred=%s gross=%v net=%v raw_bytes=%d sandbox=%v", tx.ExternalID, tx.BranchID, tx.ExternalLocationID, tx.CustomerID, tx.ExternalCustomerID, tx.CampaignID, tx.OriginalExternalID, tx.OccurredAt.Format(time.RFC3339), tx.GrossAmount, tx.NetAmount, len(tx.RawPayload), tx.Sandbox)
		result, directErr := posintegration.NewService(f.db).Ingest(t.Context(), tx)
		t.Logf("direct ingest external_id=%q result=%+v err=%v", tx.ExternalID, result, directErr)
		if index == 0 && directErr != nil {
			t.Fatalf("worker-derived valid record failed after outbox typing fix: result=%+v err=%v", result, directErr)
		}
		if index == 1 && (directErr == nil || !strings.Contains(directErr.Error(), "amounts")) {
			t.Fatalf("expected application validation for second record, got result=%+v err=%v", result, directErr)
		}
	}
	var persisted int
	if err = f.db.QueryRow(t.Context(), `SELECT count(*) FROM sales_transactions WHERE company_id=$1 AND provider='poster'`, f.company).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if persisted != 1 {
		t.Fatalf("expected first valid transaction to persist, persisted=%d", persisted)
	}
}

func TestPosterWorkerImportCannotReachPartialProgressWithCurrentIngestSQL(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	_, _ = f.db.Exec(t.Context(), `DELETE FROM outbox_events WHERE idempotency_key IN ('outbox:poster:poster-first','outbox:poster:poster-second')`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"response": []map[string]string{
			{"transaction_id": "poster-first", "date_close": "2026-08-19 10:00:00", "spot_id": "spot-1", "client_id": "client-1", "sum": "1000", "payed_sum": "1000"},
			{"transaction_id": "poster-second", "date_close": "2026-08-19 10:01:00", "spot_id": "spot-1", "client_id": "client-1", "sum": "-1000", "payed_sum": "-1000"},
		}})
	}))
	defer server.Close()
	t.Setenv("POSTER_API_BASE_URL", server.URL)
	key := integrationEncryptionKey("worker-test-secret")
	credentials, err := encryptIntegrationSecret(key, []byte(`{"accessToken":"test-token"}`))
	if err != nil {
		t.Fatal(err)
	}
	var connectionID, jobID string
	if err = f.db.QueryRow(t.Context(), `INSERT INTO integration_connections(company_id,provider,name,status,encrypted_credentials) VALUES($1,'poster','QA Poster','active',$2) RETURNING id`, f.company, credentials).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(t.Context(), `INSERT INTO integration_location_mappings(company_id,connection_id,branch_id,external_location_id,status) VALUES($1,$2,$3,'spot-1','mapped')`, f.company, connectionID, f.branch); err != nil {
		t.Fatal(err)
	}
	var customerID string
	if err = f.db.QueryRow(t.Context(), `INSERT INTO customers(company_id,first_name,phone) VALUES($1,'Poster QA','+77001112233') RETURNING id`, f.company).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(t.Context(), `INSERT INTO integration_customer_links(company_id,connection_id,customer_id,external_customer_id,status,match_method) VALUES($1,$2,$3,'client-1','linked','external_id')`, f.company, connectionID, customerID); err != nil {
		t.Fatal(err)
	}
	if err = f.db.QueryRow(t.Context(), `INSERT INTO integration_jobs(company_id,connection_id,job_type,resource) VALUES($1,$2,'poster_transactions','transactions') RETURNING id`, f.company, connectionID).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.db.Exec(context.Background(), `DELETE FROM integration_jobs WHERE id=$1`, jobID)
		_, _ = f.db.Exec(context.Background(), `DELETE FROM integration_connections WHERE id=$1`, connectionID)
	})
	err = processNextPosterJob(t.Context(), f.db, key, server.Client())
	if err == nil || !strings.Contains(err.Error(), "amounts") {
		t.Fatalf("expected second-record validation error, got %v", err)
	}
	var count int
	if err = f.db.QueryRow(t.Context(), `SELECT count(*) FROM sales_transactions WHERE company_id=$1 AND provider='poster'`, f.company).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected first transaction and outbox to commit before second failure, count=%d", count)
	}
	var status string
	var attempts int
	if err = f.db.QueryRow(t.Context(), `SELECT status,attempts FROM integration_jobs WHERE id=$1`, jobID).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || attempts != 1 {
		t.Fatalf("unexpected failed job state: %s/%d", status, attempts)
	}
}
