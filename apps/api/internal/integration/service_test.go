package integration

import (
	"testing"
	"time"
)

func TestNormalizePhone(t *testing.T) {
	tests := map[string]string{
		"+7 700 123 45 67":  "+77001234567",
		"8 (700) 123-45-67": "+77001234567",
		"7001234567":        "+77001234567",
		"invalid":           "",
	}
	for input, want := range tests {
		if got := NormalizePhone(input); got != want {
			t.Fatalf("NormalizePhone(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidateTransaction(t *testing.T) {
	valid := CanonicalTransaction{CompanyID: "company", ConnectionID: "connection", Provider: "poster", ExternalID: "receipt-1", Status: "completed", OccurredAt: time.Now(), Currency: "KZT"}
	if err := validateTransaction(valid); err != nil {
		t.Fatalf("valid transaction rejected: %v", err)
	}
	invalid := valid
	invalid.NetAmount = -1
	if err := validateTransaction(invalid); err == nil {
		t.Fatal("negative amount must be rejected")
	}
}
