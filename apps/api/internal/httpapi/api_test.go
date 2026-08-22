package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJWTRoundTripAndTampering(t *testing.T) {
	a := &api{jwtSecret: []byte("unit-test-secret")}
	want := tokenClaims{Subject: "user-1", CompanyID: "company-1", Role: "company_owner", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	token, err := a.signJWT(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.verifyJWT(token)
	if err != nil || got != want {
		t.Fatalf("JWT round trip failed: claims=%+v err=%v", got, err)
	}
	if _, err = a.verifyJWT(token + "x"); err == nil {
		t.Fatal("tampered JWT must be rejected")
	}
}

func TestExpiredJWTIsRejected(t *testing.T) {
	a := &api{jwtSecret: []byte("unit-test-secret")}
	token, err := a.signJWT(tokenClaims{Subject: "user-1", CompanyID: "company-1", Role: "employee", ExpiresAt: time.Now().Add(-time.Second).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.verifyJWT(token); err == nil {
		t.Fatal("expired JWT must be rejected")
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"email":"owner@example.com","password":"secret","admin":true}`))
	rec := httptest.NewRecorder()
	var input loginInput
	if decode(rec, req, &input) {
		t.Fatal("payload with unknown field must be rejected")
	}
	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var response envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response.Error == nil || response.Error.Code != "INVALID_JSON" {
		t.Fatalf("unexpected error envelope: body=%s err=%v", rec.Body.String(), err)
	}
}

func TestPercentage(t *testing.T) {
	if got := percentage(2, 5); got != 40 {
		t.Fatalf("percentage(2, 5) = %v, want 40", got)
	}
	if got := percentage(1, 0); got != 0 {
		t.Fatalf("percentage with zero total = %v, want 0", got)
	}
}

func TestNormalizedOutcomeDays(t *testing.T) {
	for raw, want := range map[string]int{"": 30, "6": 30, "366": 30, "invalid": 30, "7": 7, "90": 90, "365": 365} {
		if got := normalizedOutcomeDays(raw); got != want {
			t.Fatalf("normalizedOutcomeDays(%q) = %d, want %d", raw, got, want)
		}
	}
}

func TestStaffLookupIdentifiers(t *testing.T) {
	if !sixDigitCustomerCode.MatchString("004271") || sixDigitCustomerCode.MatchString("4271") {
		t.Fatal("customer code validation does not preserve the six-digit contract")
	}
	if !customerUUID.MatchString("e5b59ca2-4bb2-4f87-8b72-08da1d45ac10") {
		t.Fatal("valid customer UUID was rejected")
	}
	for _, invalid := range []string{"", "customer-1", "00000000-0000-0000-0000-000000000000", "e5b59ca2-4bb2-zzzz-8b72-08da1d45ac10"} {
		if customerUUID.MatchString(invalid) {
			t.Fatalf("invalid customer UUID %q was accepted", invalid)
		}
	}
}

func TestIntegrationSecretEncryption(t *testing.T) {
	key := integrationEncryptionKey("unit-test")
	ciphertext, err := encryptIntegrationSecret(key, []byte("webhook-secret"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := decryptIntegrationSecret(key, ciphertext)
	if err != nil || string(plaintext) != "webhook-secret" {
		t.Fatalf("integration secret round trip failed: plaintext=%q err=%v", plaintext, err)
	}
	if string(ciphertext) == "webhook-secret" {
		t.Fatal("secret was stored as plaintext")
	}
}

func TestHelpers(t *testing.T) {
	if clamp(-1, 1, 100) != 1 || clamp(101, 1, 100) != 100 || clamp(50, 1, 100) != 50 {
		t.Fatal("clamp returned an invalid value")
	}
	if !validSegment("inactive") || validSegment("unknown") {
		t.Fatal("campaign segment validation is incorrect")
	}
	if fromAddress("Tappix <noreply@tappix.kz>") != "noreply@tappix.kz" {
		t.Fatal("SMTP from address parsing is incorrect")
	}
}

func TestCampaignHoldoutIsDeterministicAndOptional(t *testing.T) {
	first := campaignHoldout("seed", "customer-42", 10)
	if campaignHoldout("seed", "customer-42", 10) != first {
		t.Fatal("holdout assignment must be deterministic")
	}
	if campaignHoldout("seed", "customer-42", 0) {
		t.Fatal("zero-percent holdout must not exclude customers")
	}
}

func TestManagerApprovalThresholds(t *testing.T) {
	tests := []struct {
		role      string
		operation string
		amount    int
		want      bool
	}{
		{"employee", "credit", 10000, false},
		{"employee", "credit", 10001, true},
		{"employee", "debit", 5000, false},
		{"employee", "debit", 5001, true},
		{"company_owner", "credit", 10001, false},
		{"company_owner", "debit", 5001, false},
	}
	for _, test := range tests {
		if got := requiresManagerApproval(test.role, test.operation, test.amount); got != test.want {
			t.Fatalf("requiresManagerApproval(%q, %q, %d) = %v, want %v", test.role, test.operation, test.amount, got, test.want)
		}
	}
}

func TestRiskRuleClassification(t *testing.T) {
	if riskRuleCode("visit.create", "Слишком частый визит") != "rapid_visit" {
		t.Fatal("visit rule was not classified")
	}
	if riskRuleCode("bonus.credit", "Крупная операция") != "large_manual_adjustment" {
		t.Fatal("bonus rule was not classified")
	}
	if riskRuleCode("bonus.debit", "Повтор ручной операции") != "duplicate_operation" {
		t.Fatal("duplicate must have priority")
	}
}

func TestWhatsAppTextDelivery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Method != http.MethodPost {
			t.Fatalf("unexpected provider request")
		}
		var payload map[string]any
		if json.NewDecoder(r.Body).Decode(&payload) != nil || payload["to"] != "77001234567" || payload["type"] != "text" {
			t.Fatalf("unexpected provider payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.test-42"}]}`))
	}))
	defer server.Close()
	t.Setenv("WHATSAPP_ACCESS_TOKEN", "test-token")
	t.Setenv("WHATSAPP_PHONE_NUMBER_ID", "phone-id")
	t.Setenv("WHATSAPP_API_BASE", server.URL)
	id, err := sendWhatsAppText(t.Context(), "+7 700 123 45 67", "Ваш подарок готов")
	if err != nil || id != "wamid.test-42" {
		t.Fatalf("delivery id=%q err=%v", id, err)
	}
}

func TestMetaWebhookSignature(t *testing.T) {
	body := []byte(`{"entry":[]}`)
	digest := hmac.New(sha256.New, []byte("app-secret"))
	_, _ = digest.Write(body)
	signature := "sha256=" + hex.EncodeToString(digest.Sum(nil))
	if !validMetaSignature("app-secret", body, signature) {
		t.Fatal("valid Meta signature rejected")
	}
	if validMetaSignature("wrong-secret", body, signature) {
		t.Fatal("invalid Meta signature accepted")
	}
}

func TestPosterWebhookFieldExtraction(t *testing.T) {
	payload := map[string]any{"event": "transaction.returned", "data": map[string]any{"transaction_id": json.Number("321")}}
	if posterString(payload, "event", "type") != "transaction.returned" || posterString(payload, "transaction_id") != "321" {
		t.Fatal("Poster webhook aliases were not normalized")
	}
}
