package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type adversarialBookingFixture struct {
	db      *pgxpool.Pool
	api     *api
	company string
	branch  string
	slug    string
}

func newAdversarialBookingFixture(t *testing.T) *adversarialBookingFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run adversarial database tests")
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	suffix := time.Now().UnixNano()
	f := &adversarialBookingFixture{db: db, api: &api{db: db}, slug: fmt.Sprintf("adversarial-booking-%d", suffix)}
	if err = db.QueryRow(t.Context(), `INSERT INTO companies(name,slug,status) VALUES('Adversarial Booking',$1,'active') RETURNING id`, f.slug).Scan(&f.company); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(t.Context(), `INSERT INTO branches(company_id,name,address,is_active) VALUES($1,'Test branch','Test address',true) RETURNING id`, f.company).Scan(&f.branch); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(t.Context(), `INSERT INTO website_settings(company_id,headline,services,published) VALUES($1,'Book now','["Consultation"]',true)`, f.company)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(t.Context(), `INSERT INTO subscriptions(company_id,plan_code,status,current_period_ends_at) VALUES($1,'pro','active',now()+interval '30 days')`, f.company)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(t.Context(), `INSERT INTO company_modules(company_id,module_code,enabled) VALUES($1,'booking',true)`, f.company)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM bookings WHERE company_id=$1`, f.company)
		_, _ = db.Exec(context.Background(), `DELETE FROM company_modules WHERE company_id=$1`, f.company)
		_, _ = db.Exec(context.Background(), `DELETE FROM website_settings WHERE company_id=$1`, f.company)
		_, _ = db.Exec(context.Background(), `DELETE FROM branches WHERE company_id=$1`, f.company)
		_, _ = db.Exec(context.Background(), `DELETE FROM companies WHERE id=$1`, f.company)
	})
	return f
}

func (f *adversarialBookingFixture) create(t *testing.T, mutate func(map[string]any)) *httptest.ResponseRecorder {
	t.Helper()
	payload := map[string]any{
		"branchId":     f.branch,
		"customerName": "Attack Client",
		"phone":        "+77001234567",
		"service":      "Consultation",
		"startsAt":     time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339),
		"comment":      "adversarial test",
	}
	if mutate != nil {
		mutate(payload)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/sites/"+f.slug+"/bookings", bytes.NewReader(body))
	req.SetPathValue("slug", f.slug)
	rec := httptest.NewRecorder()
	f.api.publicCreateBooking(rec, req)
	return rec
}

func TestAdversarialBookingRejectsDoubleBookingOfSameSlot(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	startsAt := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute).Format(time.RFC3339)
	first := f.create(t, func(p map[string]any) { p["startsAt"] = startsAt })
	if first.Code != http.StatusCreated {
		t.Fatalf("precondition: first booking returned %d: %s", first.Code, first.Body.String())
	}
	second := f.create(t, func(p map[string]any) { p["startsAt"] = startsAt })
	if second.Code != http.StatusConflict {
		t.Fatalf("same branch and slot was booked twice: second response=%d body=%s", second.Code, second.Body.String())
	}
}

func TestAdversarialBookSlotConcurrentCallsPreserveSingleBookingInvariant(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	startsAt := time.Now().UTC().Add(96 * time.Hour).Truncate(time.Minute).Format(time.RFC3339)
	const callers = 2
	ready := sync.WaitGroup{}
	ready.Add(callers)
	start := make(chan struct{})
	results := make(chan int, callers)
	workers := sync.WaitGroup{}
	workers.Add(callers)
	for range callers {
		go func() {
			defer workers.Done()
			ready.Done()
			<-start
			recorder := f.create(t, func(payload map[string]any) { payload["startsAt"] = startsAt })
			results <- recorder.Code
		}()
	}
	ready.Wait()
	close(start)
	workers.Wait()
	close(results)
	responseCodes := make([]int, 0, callers)
	for code := range results {
		responseCodes = append(responseCodes, code)
	}
	sort.Ints(responseCodes)
	var bookingCount int
	var databaseStatuses []string
	if err := f.db.QueryRow(t.Context(), `SELECT count(*),coalesce(array_agg(status ORDER BY id),'{}') FROM bookings WHERE company_id=$1 AND branch_id=$2 AND starts_at=$3`, f.company, f.branch, startsAt).Scan(&bookingCount, &databaseStatuses); err != nil {
		t.Fatal(err)
	}
	if bookingCount != 1 || len(responseCodes) != 2 || responseCodes[0] != http.StatusCreated || responseCodes[1] != http.StatusConflict {
		t.Fatalf("single-slot invariant violated: database bookings=%d statuses=%v response_codes=%v", bookingCount, databaseStatuses, responseCodes)
	}
}

func TestAdversarialBookingRejectsServiceOutsidePublishedCatalog(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	rec := f.create(t, func(p map[string]any) { p["service"] = "Attacker-controlled service" })
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("service absent from the published catalog was accepted: response=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdversarialBookingRejectsMalformedPhone(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	rec := f.create(t, func(p map[string]any) { p["phone"] = "not-a-phone" })
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("non-phone input was accepted as a phone: response=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdversarialBookingRejectsCreationAfterSiteIsUnpublished(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	if _, err := f.db.Exec(t.Context(), `UPDATE website_settings SET published=false WHERE company_id=$1`, f.company); err != nil {
		t.Fatal(err)
	}
	rec := f.create(t, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unpublished site still accepted a public booking: response=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdversarialBookingRejectsCreationWithoutBookingEntitlement(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	_, _ = f.db.Exec(t.Context(), `DELETE FROM company_modules WHERE company_id=$1 AND module_code='booking'`, f.company)
	// No enabled company_modules row exists for booking. A public URL must not
	// bypass the same commercial entitlement enforced on the protected API.
	rec := f.create(t, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("company without booking entitlement accepted a booking: response=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdversarialBookingRejectsCancelledToCompletedTransition(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	var bookingID string
	if err := f.db.QueryRow(t.Context(), `INSERT INTO bookings(company_id,branch_id,customer_name,phone,service,starts_at,status) VALUES($1,$2,'Attack Client','+77001234567','Consultation',now()+interval '1 day','cancelled') RETURNING id`, f.company, f.branch).Scan(&bookingID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/bookings/"+bookingID, bytes.NewBufferString(`{"status":"completed"}`))
	req.SetPathValue("id", bookingID)
	req = req.WithContext(context.WithValue(req.Context(), identityKey, tokenClaims{CompanyID: f.company, Role: "company_owner"}))
	rec := httptest.NewRecorder()
	f.api.updateBooking(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("cancelled booking was resurrected as completed: response=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdversarialRegistrationCannotResetExistingCustomerPIN(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("set TEST_REDIS_ADDR to run session-backed adversarial tests")
	}
	f.api.redis = redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = f.api.redis.Close() })
	f.api.jwtSecret = []byte("adversarial-test-secret")
	var deviceToken string
	if err := f.db.QueryRow(t.Context(), `INSERT INTO devices(company_id,branch_id,kind,name,destination) VALUES($1,$2,'qr','Public registration QR','join') RETURNING token`, f.company, f.branch).Scan(&deviceToken); err != nil {
		t.Fatal(err)
	}
	oldHash, err := bcrypt.GenerateFromPassword([]byte("victim-pin"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	var customerID string
	if err = f.db.QueryRow(t.Context(), `INSERT INTO customers(company_id,first_name,last_name,phone,pin_hash) VALUES($1,'Victim','Customer','+77009990000',$2) RETURNING id`, f.company, string(oldHash)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.db.Exec(context.Background(), `DELETE FROM customers WHERE id=$1`, customerID)
		_, _ = f.db.Exec(context.Background(), `DELETE FROM devices WHERE token=$1`, deviceToken)
	})
	payload := map[string]any{"token": deviceToken, "firstName": "Attacker", "lastName": "Owned", "phone": "+77009990000", "pin": "attacker-pin", "consent": true}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	f.api.customerRegister(rec, req)
	var storedHash string
	if err = f.db.QueryRow(t.Context(), `SELECT pin_hash FROM customers WHERE id=$1`, customerID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if rec.Code == http.StatusCreated && bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("attacker-pin")) == nil {
		t.Fatalf("registration QR reset an existing customer's PIN and issued a session: response=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdversarialCustomerPINLoginHasNoBruteForceLockout(t *testing.T) {
	f := newAdversarialBookingFixture(t)
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("set TEST_REDIS_ADDR to run session-backed adversarial tests")
	}
	f.api.redis = redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = f.api.redis.Close() })
	f.api.jwtSecret = []byte("adversarial-test-secret")
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-pin"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	var customerID string
	if err = f.db.QueryRow(t.Context(), `INSERT INTO customers(company_id,first_name,last_name,phone,pin_hash) VALUES($1,'Victim','Customer','+77008880000',$2) RETURNING id`, f.company, string(hash)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = f.db.Exec(context.Background(), `DELETE FROM customers WHERE id=$1`, customerID) })
	login := func(pin string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"company": f.slug, "phone": "+77008880000", "pin": pin})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/customer/login", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		f.api.customerLogin(rec, req)
		return rec
	}
	for attempt := 0; attempt < 25; attempt++ {
		if rec := login(fmt.Sprintf("wrong-%d", attempt)); rec.Code != http.StatusUnauthorized && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("unexpected wrong-PIN response %d: %s", rec.Code, rec.Body.String())
		}
	}
	rec := login("correct-pin")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("25 failed PIN guesses caused no lockout; the next guess authenticated: response=%d body=%s", rec.Code, rec.Body.String())
	}
}
