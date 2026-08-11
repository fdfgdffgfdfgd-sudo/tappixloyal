package httpapi

import (
	"reflect"
	"testing"
)

func TestNormalizePlanCode(t *testing.T) {
	tests := map[string]string{
		"Starter":    "starter",
		" growth ":   "growth",
		"Business":   "growth",
		"PRO":        "pro",
		"enterprise": "pro",
		"unknown":    "",
	}
	for input, expected := range tests {
		if actual := normalizePlanCode(input); actual != expected {
			t.Errorf("normalizePlanCode(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestDefaultModulesForPlan(t *testing.T) {
	tests := map[string][]string{
		"starter": {"core", "crm", "loyalty", "reviews"},
		"growth":  {"core", "crm", "loyalty", "reviews", "analytics", "website", "booking", "email", "sms", "telegram", "partnerships"},
		"pro":     {"core", "crm", "loyalty", "reviews", "analytics", "website", "booking", "email", "sms", "telegram", "partnerships", "api"},
	}
	for plan, expected := range tests {
		if actual := defaultModulesForPlan(plan); !reflect.DeepEqual(actual, expected) {
			t.Errorf("defaultModulesForPlan(%q) = %#v, want %#v", plan, actual, expected)
		}
	}
}
