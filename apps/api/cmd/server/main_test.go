package main

import "testing"

func TestProductionEnvironmentFailsClosed(t *testing.T) {
	values := map[string]string{"APP_ENV": "production", "OTP_DEV_MODE": "true"}
	if validateEnvironment(func(key string) string { return values[key] }) == nil {
		t.Fatal("unsafe production environment must fail")
	}
}

func TestLocalEnvironmentKeepsDeveloperDefaults(t *testing.T) {
	if err := validateEnvironment(func(string) string { return "" }); err != nil {
		t.Fatalf("local environment rejected: %v", err)
	}
}

func TestValidProductionEnvironment(t *testing.T) {
	values := map[string]string{"APP_ENV": "production", "DATABASE_URL": "postgres://tappix:7gK9pL2vR4xN6mQ8@db:5432/tappix?sslmode=require", "JWT_SECRET": "d1f4a7b0c3e6h9k2m5p8s1v4y7B0E3H6", "APP_URL": "https://app.tappix.kz", "SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": "tappix", "SMTP_PASSWORD": "strong-password", "SMTP_TLS": "true", "WHATSAPP_ACCESS_TOKEN": "token", "WHATSAPP_PHONE_NUMBER_ID": "phone", "METRICS_TOKEN": "metrics-token", "OTP_DEV_MODE": "false"}
	if err := validateEnvironment(func(key string) string { return values[key] }); err != nil {
		t.Fatalf("valid production environment rejected: %v", err)
	}
}
