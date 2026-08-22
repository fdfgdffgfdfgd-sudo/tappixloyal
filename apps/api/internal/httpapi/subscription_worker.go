package httpapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// StartSubscriptionLifecycleWorker makes expiry a persisted commercial state,
// rather than merely interpreting an old date differently in each handler.
func StartSubscriptionLifecycleWorker(ctx context.Context, db *pgxpool.Pool) {
	run := func() {
		if _, err := db.Exec(ctx, `UPDATE subscriptions SET status='expired',updated_at=now()
			WHERE status IN('trial','active') AND current_period_ends_at IS NOT NULL AND current_period_ends_at<=now()`); err != nil && ctx.Err() == nil {
			slog.Error("subscription lifecycle sweep failed", "error", err)
		}
	}
	run()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
