package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tappix/platform/apps/api/internal/httpapi"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
	redisClient := redis.NewClient(&redis.Options{Addr: env("REDIS_ADDR", "localhost:6379")})
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

func startWorkersWhenSchemaReady(ctx context.Context, db *pgxpool.Pool, secret string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		var ready bool
		err := db.QueryRow(ctx, `SELECT to_regclass('public.integration_jobs') IS NOT NULL
			AND to_regclass('public.campaign_automations') IS NOT NULL
			AND to_regclass('public.analytics_daily_facts') IS NOT NULL`).Scan(&ready)
		if err == nil && ready {
			httpapi.StartAutomation(ctx, db)
			httpapi.StartIntegrationWorkers(ctx, db, secret)
			httpapi.StartAnalyticsProjectionWorker(ctx, db)
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
