package httpapi

import (
	"encoding/json"
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
