package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIngestCharacterizesInsertParameterInference(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run database characterization")
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	suffix := time.Now().UnixNano()
	var company, connection, customer, branch string
	if err = db.QueryRow(t.Context(), `INSERT INTO companies(name,slug,status) VALUES('Ingest Characterization',$1,'active') RETURNING id`, "ingest-characterization-"+strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "")).Scan(&company); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(t.Context(), `INSERT INTO integration_connections(company_id,provider,name,status) VALUES($1,'poster','Characterization','active') RETURNING id`, company).Scan(&connection); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(t.Context(), `INSERT INTO branches(company_id,name,address,is_active) VALUES($1,'Characterization branch','Test',true) RETURNING id`, company).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(t.Context(), `INSERT INTO customers(company_id,first_name,phone) VALUES($1,'Known Customer',$2) RETURNING id`, company, "+77005550000").Scan(&customer); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(t.Context(), `INSERT INTO integration_location_mappings(company_id,connection_id,branch_id,external_location_id,status) VALUES($1,$2,$3,'spot-1','mapped')`, company, connection, branch); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(t.Context(), `INSERT INTO integration_customer_links(company_id,connection_id,customer_id,external_customer_id,status,match_method) VALUES($1,$2,$3,'client-1','linked','external_id')`, company, connection, customer); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM companies WHERE id=$1`, company) })
	_, _ = db.Exec(t.Context(), `DELETE FROM outbox_events WHERE idempotency_key LIKE 'outbox:poster:case-%'`)
	_ = suffix
	base := CanonicalTransaction{CompanyID: company, ConnectionID: connection, Provider: "poster", ExternalID: "case-base", Status: "completed", OccurredAt: time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC), GrossAmount: 100, CashPaidAmount: 100, NetAmount: 100, Currency: "KZT", Source: "characterization", Sandbox: true}
	cases := []struct {
		name    string
		mutate  func(*CanonicalTransaction)
		wantErr bool
		errPart string
	}{
		{name: "minimal optional null customer empty", mutate: func(in *CanonicalTransaction) { in.CustomerID = "" }, wantErr: false},
		{name: "known customer uuid", mutate: func(in *CanonicalTransaction) { in.CustomerID = customer }, wantErr: false},
		{name: "branch and known customer", mutate: func(in *CanonicalTransaction) { in.BranchID = branch; in.CustomerID = customer }, wantErr: false},
		{name: "branch with empty customer", mutate: func(in *CanonicalTransaction) { in.BranchID = branch; in.CustomerID = "" }, wantErr: false},
		{name: "external branch and customer resolution", mutate: func(in *CanonicalTransaction) { in.ExternalLocationID = "spot-1"; in.ExternalCustomerID = "client-1" }, wantErr: false},
		{name: "normal production path with outbox", mutate: func(in *CanonicalTransaction) {
			in.ExternalLocationID = "spot-1"
			in.ExternalCustomerID = "client-1"
			in.Sandbox = false
		}, wantErr: false},
		{name: "empty campaign with known customer", mutate: func(in *CanonicalTransaction) { in.CustomerID = customer; in.CampaignID = "" }, wantErr: false},
		{name: "original external id absent", mutate: func(in *CanonicalTransaction) { in.CustomerID = customer; in.OriginalExternalID = "" }, wantErr: false},
		{name: "cost amount null", mutate: func(in *CanonicalTransaction) { in.CustomerID = customer; in.CostAmount = nil }, wantErr: false},
		{name: "decimal amounts", mutate: func(in *CanonicalTransaction) {
			in.CustomerID = customer
			in.GrossAmount = 12.34
			in.CashPaidAmount = 12.34
			in.NetAmount = 12.34
		}, wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			in.ExternalID = "case-" + strings.ReplaceAll(tc.name, " ", "-")
			tc.mutate(&in)
			result, gotErr := NewService(db).Ingest(t.Context(), in)
			if tc.wantErr {
				if gotErr == nil || !strings.Contains(gotErr.Error(), tc.errPart) {
					t.Fatalf("expected %q, got result=%+v err=%v", tc.errPart, result, gotErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("unexpected ingest error: %v", gotErr)
			}
			var count int
			if err = db.QueryRow(t.Context(), `SELECT count(*) FROM sales_transactions WHERE id=$1`, result.TransactionID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("expected one persisted transaction, got %d", count)
			}
		})
	}
}
