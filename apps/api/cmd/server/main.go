package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tappix/platform/apps/api/internal/httpapi"
)

func main() {
	configureLogging()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := validateEnvironment(os.Getenv); err != nil {
		slog.Error("production configuration invalid", "error", err)
		os.Exit(1)
	}

	db, err := pgxpool.New(ctx, env("DATABASE_URL", "postgres://tappix:tappix_local@localhost:5432/tappix?sslmode=disable"))
	if err != nil {
		slog.Error("database config failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err = db.Ping(ctx); err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: env("REDIS_ADDR", "localhost:6379"), Password: os.Getenv("REDIS_PASSWORD")})
	defer redisClient.Close()
	if err = redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("redis unavailable", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: env("HTTP_ADDR", ":8080"), Handler: httpapi.New(db, redisClient, env("JWT_SECRET", "local-dev-secret-change-me")), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		slog.Info("tappix api started", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "error", err)
			stop()
		}
	}()
	go startWorkersWhenSchemaReady(ctx, db, env("JWT_SECRET", "local-dev-secret-change-me"))
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}

func configureLogging() {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "debug":
		if !strings.EqualFold(os.Getenv("APP_ENV"), "production") {
			level = slog.LevelDebug
		}
	}
	options := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, options)))
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, options)))
}

func validateEnvironment(getenv func(string) string) error {
	if strings.ToLower(strings.TrimSpace(getenv("APP_ENV"))) != "production" {
		return nil
	}
	required := []string{"DATABASE_URL", "REDIS_ADDR", "REDIS_PASSWORD", "JWT_SECRET", "INTEGRATION_ENCRYPTION_KEY", "APP_URL", "WEB_ORIGIN", "METRICS_TOKEN", "RELEASE_SHA"}
	missing := []string{}
	for _, key := range required {
		if strings.TrimSpace(getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required production variables: %s", strings.Join(missing, ", "))
	}
	for _, key := range []string{"DATABASE_URL", "REDIS_PASSWORD", "JWT_SECRET", "INTEGRATION_ENCRYPTION_KEY", "SMTP_PASSWORD"} {
		value := strings.ToLower(getenv(key))
		if strings.Contains(value, "admin2026!") || strings.Contains(value, "tappix2026!") || strings.Contains(value, "docmed2026!") {
			return fmt.Errorf("%s contains a demo credential", key)
		}
	}
	if strings.EqualFold(getenv("OTP_DEV_MODE"), "true") {
		return fmt.Errorf("OTP_DEV_MODE must be false in production")
	}
	if err := requireCompleteGroup(getenv, "SMTP", []string{"SMTP_HOST", "SMTP_FROM", "SMTP_USERNAME", "SMTP_PASSWORD"}); err != nil {
		return err
	}
	if strings.TrimSpace(getenv("SMTP_HOST")) != "" && !strings.EqualFold(getenv("SMTP_TLS"), "true") {
		return fmt.Errorf("SMTP_TLS must be true when SMTP is configured in production")
	}
	if err := requireCompleteGroup(getenv, "WhatsApp", []string{"WHATSAPP_ACCESS_TOKEN", "WHATSAPP_PHONE_NUMBER_ID", "WHATSAPP_APP_SECRET", "WHATSAPP_VERIFY_TOKEN"}); err != nil {
		return err
	}
	secret := getenv("JWT_SECRET")
	lower := strings.ToLower(secret)
	if len(secret) < 32 || strings.Contains(lower, "change") || strings.Contains(lower, "secret") || strings.Contains(lower, "development") {
		return fmt.Errorf("JWT_SECRET must be a strong unique value of at least 32 characters")
	}
	database := strings.ToLower(getenv("DATABASE_URL"))
	if strings.Contains(database, "tappix_local") || strings.Contains(database, "changeme") || strings.Contains(database, "password") {
		return fmt.Errorf("DATABASE_URL contains an unsafe production credential")
	}
	if !strings.Contains(database, "sslmode=require") && !strings.Contains(database, "sslmode=verify-") {
		return fmt.Errorf("DATABASE_URL must require TLS in production")
	}
	appURL := strings.ToLower(getenv("APP_URL"))
	if !strings.HasPrefix(appURL, "https://") {
		return fmt.Errorf("APP_URL must use HTTPS in production")
	}
	if strings.TrimSuffix(strings.ToLower(getenv("WEB_ORIGIN")), "/") != strings.TrimSuffix(appURL, "/") {
		return fmt.Errorf("WEB_ORIGIN must match APP_URL in the same-origin production topology")
	}
	return nil
}

func requireCompleteGroup(getenv func(string) string, name string, keys []string) error {
	configured := false
	missing := []string{}
	for _, key := range keys {
		if strings.TrimSpace(getenv(key)) == "" {
			missing = append(missing, key)
		} else {
			configured = true
		}
	}
	if configured && len(missing) > 0 {
		return fmt.Errorf("%s configuration is incomplete; missing: %s", name, strings.Join(missing, ", "))
	}
	return nil
}

func startWorkersWhenSchemaReady(ctx context.Context, db *pgxpool.Pool, secret string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		var ready bool
		err := db.QueryRow(ctx, `SELECT to_regclass('public.integration_jobs') IS NOT NULL
			AND to_regclass('public.campaign_automations') IS NOT NULL
			AND to_regclass('public.analytics_daily_facts') IS NOT NULL
			AND to_regclass('public.reward_transactions') IS NOT NULL`).Scan(&ready)
		if err == nil && ready {
			httpapi.StartAutomation(ctx, db)
			httpapi.StartIntegrationWorkers(ctx, db, secret)
			httpapi.StartAnalyticsProjectionWorker(ctx, db)
			httpapi.StartReportWorker(ctx, db, secret)
			httpapi.StartRewardExpiryWorker(ctx, db)
			go httpapi.StartSubscriptionLifecycleWorker(ctx, db)
			httpapi.MarkWorkersReady()
			slog.Info("background workers started after schema became ready")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
