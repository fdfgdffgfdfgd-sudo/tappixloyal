package main

import (
	"strings"
	"testing"
)

// A configuration that is safe to run in production. Each guard test starts
// from this and spoils exactly one value, so a failure names the guard that
// stopped caring.
func productionDefaults() map[string]string {
	return map[string]string{
		"APP_ENV":                  "production",
		"DATABASE_URL":             "postgres://tappix:7gK9pL2vR4xN6mQ8@db:5432/tappix?sslmode=require",
		"JWT_SECRET":               "d1f4a7b0c3e6h9k2m5p8s1v4y7B0E3H6",
		"APP_URL":                  "https://app.tappix.kz",
		"SMTP_HOST":                "smtp.example.com",
		"SMTP_USERNAME":            "tappix",
		"SMTP_PASSWORD":            "strong-password",
		"SMTP_TLS":                 "true",
		"WHATSAPP_ACCESS_TOKEN":    "token",
		"WHATSAPP_PHONE_NUMBER_ID": "phone",
		"METRICS_TOKEN":            "metrics-token",
		"OTP_DEV_MODE":             "false",
	}
}

func lookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestValidProductionEnvironment(t *testing.T) {
	if err := validateEnvironment(lookup(productionDefaults())); err != nil {
		t.Fatalf("valid production environment rejected: %v", err)
	}
}

func TestLocalEnvironmentKeepsDeveloperDefaults(t *testing.T) {
	if err := validateEnvironment(func(string) string { return "" }); err != nil {
		t.Fatalf("local environment rejected: %v", err)
	}
}

func TestProductionGuards(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{"OTP left in development mode", "OTP_DEV_MODE", "true", "OTP_DEV_MODE"},
		{"mail sent without TLS", "SMTP_TLS", "false", "SMTP_TLS"},
		{"signing secret too short", "JWT_SECRET", "short", "JWT_SECRET"},
		{"signing secret still says secret", "JWT_SECRET", "my-super-secret-value-long-enough", "JWT_SECRET"},
		{"signing secret still says change", "JWT_SECRET", "please-change-this-value-before-launch", "JWT_SECRET"},
		{"database keeps the local password", "DATABASE_URL", "postgres://tappix:tappix_local@db:5432/tappix", "DATABASE_URL"},
		{"public address is not HTTPS", "APP_URL", "http://app.tappix.kz", "APP_URL"},
		{"metrics token missing", "METRICS_TOKEN", "", "METRICS_TOKEN"},
		{"database url missing", "DATABASE_URL", "", "DATABASE_URL"},
		{"whatsapp token missing", "WHATSAPP_ACCESS_TOKEN", "", "WHATSAPP_ACCESS_TOKEN"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			values := productionDefaults()
			values[c.key] = c.value
			err := validateEnvironment(lookup(values))
			if err == nil {
				t.Fatalf("unsafe production configuration accepted: %s=%q", c.key, c.value)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error does not name %s, operator cannot act on it: %v", c.want, err)
			}
		})
	}
}

// Outside production the same values must not block a developer's machine.
func TestGuardsOnlyApplyToProduction(t *testing.T) {
	values := productionDefaults()
	values["APP_ENV"] = "development"
	values["OTP_DEV_MODE"] = "true"
	values["JWT_SECRET"] = "local-dev-secret-change-before-production"
	values["APP_URL"] = "http://localhost:8088"
	if err := validateEnvironment(lookup(values)); err != nil {
		t.Fatalf("development environment rejected: %v", err)
	}
}
