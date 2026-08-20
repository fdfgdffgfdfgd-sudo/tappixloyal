// Command testseed creates disposable fixtures for local load testing only.
// It is intentionally not wired into the production server or deployment.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type manifest struct {
	Version             int       `json:"version"`
	GeneratedAt         time.Time `json:"generatedAt"`
	BaseURL             string    `json:"baseUrl"`
	Email               []string  `json:"emails"`
	Password            string    `json:"password"`
	Companies           []string  `json:"companyIds"`
	Branches            []string  `json:"branchIds"`
	Guests              []string  `json:"guestTokens"`
	Slugs               []string  `json:"siteSlugs"`
	Slots               []string  `json:"bookingSlots"`
	UserCount           int       `json:"userCount"`
	CompanyCount        int       `json:"companyCount"`
	GuestFixtureCount   int       `json:"guestFixtureCount"`
	BookingFixtureCount int       `json:"bookingFixtureCount"`
}

func main() {
	if os.Getenv("TAPPIX_TEST_ENV") != "1" || strings.Contains(strings.ToLower(os.Getenv("DATABASE_URL")), "prod") {
		fmt.Fprintln(os.Stderr, "refusing to seed: set TAPPIX_TEST_ENV=1 and use a local/test database")
		os.Exit(2)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	n := flag.Int("users", 100, "users per company")
	companies := flag.Int("companies", 3, "companies")
	manifestPath := flag.String("manifest", "", "atomic output manifest path")
	flag.Parse()
	if *n < 1 || *n > 500 || *companies < 1 || *companies > 10 {
		fmt.Fprintln(os.Stderr, "users must be 1..500 and companies 1..10")
		os.Exit(2)
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	password := "LoadTest-Only-2026!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	out := manifest{Version: 1, GeneratedAt: time.Now().UTC(), BaseURL: os.Getenv("TAPPIX_TEST_BASE_URL"), Password: password, CompanyCount: *companies, UserCount: *n * *companies}
	ctx := context.Background()
	for c := 0; c < *companies; c++ {
		suffix := randomSuffix()
		var company, branch string
		if err = db.QueryRow(ctx, `INSERT INTO companies(name,slug,status) VALUES($1,$2,'active') RETURNING id`, "Load Test "+suffix, "load-test-"+suffix).Scan(&company); err != nil {
			panic(err)
		}
		if err = db.QueryRow(ctx, `INSERT INTO branches(company_id,name,address,is_active) VALUES($1,'Load Test Branch','Local',true) RETURNING id`, company).Scan(&branch); err != nil {
			panic(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO subscriptions(company_id,plan_code,status,amount,current_period_ends_at) VALUES($1,'pro','active',0,now()+interval '30 days')`, company); err != nil {
			panic(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO company_modules(company_id,module_code,enabled) SELECT $1,code,true FROM modules WHERE code IN ('core','crm','loyalty','booking','website') ON CONFLICT DO NOTHING`, company); err != nil {
			panic(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO website_settings(company_id,headline,services,published) VALUES($1,'Load test site','["Consultation"]',true)`, company); err != nil {
			panic(err)
		}
		var reward string
		if err = db.QueryRow(ctx, `INSERT INTO reward_definitions(company_id,name,description,reward_type,value,created_by) VALUES($1,'Load test reward','Disposable local reward','gift',0,(SELECT id FROM users WHERE company_id=$1 ORDER BY created_at LIMIT 1)) RETURNING id`, company).Scan(&reward); err != nil {
			panic(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO reward_rules(company_id,definition_id,event_type,threshold) VALUES($1,$2,'visit_milestone',6)`, company, reward); err != nil {
			panic(err)
		}
		var device string
		if err = db.QueryRow(ctx, `INSERT INTO devices(company_id,branch_id,kind,name,destination) VALUES($1,$2,'qr','Load test QR','join') RETURNING token`, company, branch).Scan(&device); err != nil {
			panic(err)
		}
		out.Companies = append(out.Companies, company)
		out.Slugs = append(out.Slugs, "load-test-"+suffix)
		for i := 1; i <= *n; i++ {
			out.Slots = append(out.Slots, time.Now().UTC().Add(time.Duration(c*30+i)*time.Hour).Format(time.RFC3339))
		}
		out.Branches = append(out.Branches, branch)
		out.Guests = append(out.Guests, device)
		for u := 0; u < *n; u++ {
			email := fmt.Sprintf("load+%s-%03d@example.test", suffix, u)
			if _, err = db.Exec(ctx, `INSERT INTO users(company_id,branch_id,first_name,email,password_hash,role,status) VALUES($1,$2,$3,$4,$5,'company_owner','active')`, company, branch, "Load User", email, string(hash)); err != nil {
				panic(err)
			}
			out.Email = append(out.Email, email)
		}
		if _, err = db.Exec(ctx, `INSERT INTO company_memberships(company_id,user_id,role,status) SELECT $1,id,'owner','active' FROM users WHERE company_id=$1`, company); err != nil {
			panic(err)
		}
	}
	out.GuestFixtureCount = len(out.Guests)
	out.BookingFixtureCount = len(out.Slots)
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if *manifestPath != "" {
		tmp := *manifestPath + ".tmp"
		if err = os.WriteFile(tmp, enc, 0600); err != nil {
			panic(err)
		}
		if err = os.Rename(tmp, *manifestPath); err != nil {
			panic(err)
		}
	}
	fmt.Println(string(enc))
}

func randomSuffix() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}
