package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStaffCustomerLookupSupportsCodeQRAndPhone(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run staff mode database tests")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var company, staff, customer, definition, rule string
	if err = db.QueryRow(t.Context(), `INSERT INTO companies(name,slug) VALUES('Staff Mode QA','staff-mode-'||replace(gen_random_uuid()::text,'-','')) RETURNING id`).Scan(&company); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM companies WHERE id=$1`, company) })
	if err = db.QueryRow(t.Context(), `INSERT INTO users(company_id,first_name,email,password_hash,role,status) VALUES($1,'Cashier','cashier-'||gen_random_uuid()::text||'@example.com','test','employee','active') RETURNING id`, company).Scan(&staff); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(t.Context(), `INSERT INTO customers(company_id,first_name,last_name,phone,customer_code,total_points,total_visits) VALUES($1,'Мадина','Тест','+7 700 555 44 33','482731',350,5) RETURNING id`, company).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(t.Context(), `INSERT INTO reward_definitions(company_id,name,description,reward_type,value,created_by) VALUES($1,'Бесплатная чистка','','gift',0,$2) RETURNING id`, company, staff).Scan(&definition); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(t.Context(), `INSERT INTO reward_rules(company_id,definition_id,event_type,threshold) VALUES($1,$2,'visit_created',6) RETURNING id`, company, definition).Scan(&rule); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(t.Context(), `INSERT INTO customer_reward_progress(company_id,customer_id,rule_id,current_value,target_value,status,cycle_key) VALUES($1,$2,$3,5,6,'in_progress','lifetime')`, company, customer, rule); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(t.Context(), `INSERT INTO customer_rewards(company_id,customer_id,definition_id,name,status) VALUES($1,$2,$3,'Приветственный подарок','available')`, company, customer, definition); err != nil {
		t.Fatal(err)
	}

	a := &api{db: db}
	for name, payload := range map[string]string{
		"code":  `{"code":"482731"}`,
		"qr":    `{"customerId":"` + customer + `"}`,
		"phone": `{"phone":"+7 700 555 44 33"}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/staff/customers/lookup", strings.NewReader(payload))
			req = req.WithContext(context.WithValue(req.Context(), identityKey, tokenClaims{Subject: staff, CompanyID: company, Role: "employee"}))
			res := httptest.NewRecorder()
			a.customerByCode(res, req)
			if res.Code != http.StatusOK {
				t.Fatalf("lookup returned %d: %s", res.Code, res.Body.String())
			}
			var body struct {
				Success bool `json:"success"`
				Data    struct {
					ID             string           `json:"id"`
					PhoneMasked    string           `json:"phoneMasked"`
					RewardProgress []map[string]any `json:"rewardProgress"`
					Rewards        []map[string]any `json:"rewards"`
				} `json:"data"`
			}
			if json.Unmarshal(res.Body.Bytes(), &body) != nil || !body.Success || body.Data.ID != customer || len(body.Data.RewardProgress) != 1 || len(body.Data.Rewards) != 1 {
				t.Fatalf("staff context is incomplete: %s", res.Body.String())
			}
			if strings.Contains(body.Data.PhoneMasked, "5554433") {
				t.Fatalf("staff response exposed the full phone: %q", body.Data.PhoneMasked)
			}
		})
	}
}
