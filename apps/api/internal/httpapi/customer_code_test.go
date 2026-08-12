package httpapi

import "testing"

func TestSixDigitCustomerCode(t *testing.T) {
	valid := []string{"000000", "482731", "999999"}
	invalid := []string{"48273", "4827310", "482 731", "abcdef", ""}
	for _, code := range valid {
		if !sixDigitCustomerCode.MatchString(code) {
			t.Fatalf("expected %q to be valid", code)
		}
	}
	for _, code := range invalid {
		if sixDigitCustomerCode.MatchString(code) {
			t.Fatalf("expected %q to be invalid", code)
		}
	}
}
