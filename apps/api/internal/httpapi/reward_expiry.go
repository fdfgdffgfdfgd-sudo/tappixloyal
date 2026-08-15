package httpapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rewardExpiryInterval keeps the stored status close to what the interface
// already shows. Staff never wait on this sweep — the reward list and the
// redemption check both resolve a run-out reservation at read time — so this
// only bounds how long the audit trail and the customer timeline lag behind.
// Reservations can be as short as a minute, so some lag is unavoidable here.
const rewardExpiryInterval = 5 * time.Minute

// rewardExpiryBatch bounds one company's sweep so a large backlog cannot hold a
// transaction open across the whole platform.
const rewardExpiryBatch = 200

// StartRewardExpiryWorker releases reservations that ran out and closes rewards
// past their validity date.
//
// Both transitions used to happen only in POST /rewards/expire, which is
// permission-gated and called by hand. Nothing called it, so the reward list and
// the redemption check papered over the stale rows at read time while the
// database kept them 'reserved' or 'available' forever: the release never
// reached the reward's audit trail, and a customer whose reward ran out never
// got the reward.expired entry on their timeline.
func StartRewardExpiryWorker(ctx context.Context, db *pgxpool.Pool) {
	run := func() {
		released, expired, err := processRewardExpiry(ctx, db, "")
		if err != nil {
			slog.Error("reward expiry worker failed", "error", err)
			return
		}
		if released > 0 || expired > 0 {
			slog.Info("reward expiry worker swept", "reservationsReleased", released, "expired", expired)
		}
	}
	go func() {
		run()
		ticker := time.NewTicker(rewardExpiryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

// processRewardExpiry sweeps every company with work waiting, or just one when
// onlyCompany is set. Each company commits on its own so one failing tenant
// cannot discard the work already done for the others.
func processRewardExpiry(ctx context.Context, db *pgxpool.Pool, onlyCompany string) (int, int, error) {
	companies, err := companiesAwaitingRewardExpiry(ctx, db, onlyCompany)
	if err != nil {
		return 0, 0, err
	}
	released, expired := 0, 0
	var firstErr error
	for _, company := range companies {
		companyReleased, companyExpired, err := sweepCompanyRewardExpiry(ctx, db, company, nil)
		released += companyReleased
		expired += companyExpired
		if err != nil {
			slog.Error("reward expiry sweep failed for company", "companyId", company, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return released, expired, firstErr
}

func companiesAwaitingRewardExpiry(ctx context.Context, db *pgxpool.Pool, onlyCompany string) ([]string, error) {
	rows, err := db.Query(ctx, `SELECT DISTINCT company_id::text FROM customer_rewards
		WHERE ($1='' OR company_id::text=$1)
		AND ((status='reserved' AND reserved_until<=now()) OR (status IN ('available','reserved') AND expires_at<=now()))`, onlyCompany)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	companies := []string{}
	for rows.Next() {
		var company string
		if err := rows.Scan(&company); err != nil {
			return nil, err
		}
		companies = append(companies, company)
	}
	return companies, rows.Err()
}

// sweepCompanyRewardExpiry runs both transitions for one company in a single
// transaction. actor is the user who asked for it, or nil for the worker — the
// reward history renders a missing actor as "System".
func sweepCompanyRewardExpiry(ctx context.Context, db *pgxpool.Pool, company string, actor any) (int, int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	released, expired, err := sweepRewardExpiry(ctx, tx, company, actor)
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return released, expired, nil
}

// sweepRewardExpiry is the one implementation of both transitions, shared by the
// worker and by POST /rewards/expire so the two can never drift apart.
func sweepRewardExpiry(ctx context.Context, tx pgx.Tx, company string, actor any) (int, int, error) {
	released, err := releaseRanOutReservations(ctx, tx, company, actor)
	if err != nil {
		return 0, 0, err
	}
	expired, err := closeRewardsPastValidity(ctx, tx, company, actor)
	if err != nil {
		return len(released), 0, err
	}
	return len(released), len(expired), nil
}

// sweptReward carries the status the reward held before the sweep, because the
// audit trail is meant to say what actually happened: a reward can reach its
// validity date while still reserved, and recording that as a release from
// 'available' would be the same kind of convenient fiction this worker exists
// to stop telling.
type sweptReward struct{ id, customerID, previousStatus string }

// releaseRanOutReservations hands a reward back to the pool once nobody
// completed the handover in time. Rewards already past their validity date are
// left alone — closeRewardsPastValidity ends those instead.
func releaseRanOutReservations(ctx context.Context, tx pgx.Tx, company string, actor any) ([]sweptReward, error) {
	items, err := collectSweptRewards(ctx, tx, `UPDATE customer_rewards cr SET status='available',reserved_at=NULL,reserved_until=NULL,reserved_by=NULL
		FROM (
			SELECT id,status FROM customer_rewards
			WHERE company_id=$1 AND status='reserved' AND reserved_until<=now() AND (expires_at IS NULL OR expires_at>now())
			ORDER BY reserved_until LIMIT $2 FOR UPDATE SKIP LOCKED
		) src WHERE cr.id=src.id RETURNING cr.id,cr.customer_id,src.status`, company)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `INSERT INTO reward_transactions(company_id,reward_id,customer_id,actor_id,operation,from_status,to_status,reason)
			VALUES($1,$2,$3,$4,'reservation_released',$5,'available','Истёк срок резерва')`, company, item.id, item.customerID, actor, item.previousStatus); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// closeRewardsPastValidity ends rewards the customer can no longer use, and
// tells the customer so through their timeline.
func closeRewardsPastValidity(ctx context.Context, tx pgx.Tx, company string, actor any) ([]sweptReward, error) {
	items, err := collectSweptRewards(ctx, tx, `UPDATE customer_rewards cr SET status='expired'
		FROM (
			SELECT id,status FROM customer_rewards
			WHERE company_id=$1 AND status IN ('available','reserved') AND expires_at<=now()
			ORDER BY expires_at LIMIT $2 FOR UPDATE SKIP LOCKED
		) src WHERE cr.id=src.id RETURNING cr.id,cr.customer_id,src.status`, company)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		var name string
		if err := tx.QueryRow(ctx, `SELECT coalesce(d.name,cr.name,'Награда') FROM customer_rewards cr
			LEFT JOIN reward_definitions d ON d.id=cr.definition_id AND d.company_id=cr.company_id
			WHERE cr.company_id=$1 AND cr.id=$2`, company, item.id).Scan(&name); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO reward_transactions(company_id,reward_id,customer_id,actor_id,operation,from_status,to_status,reason)
			VALUES($1,$2,$3,$4,'expired',$5,'expired','Истёк срок действия')`, company, item.id, item.customerID, actor, item.previousStatus); err != nil {
			return nil, err
		}
		if err := appendCustomerEventCtx(ctx, tx, company, item.customerID, "reward.expired", "", "reward-expired:"+item.id, map[string]any{"rewardId": item.id, "name": name}); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func collectSweptRewards(ctx context.Context, tx pgx.Tx, query, company string) ([]sweptReward, error) {
	rows, err := tx.Query(ctx, query, company, rewardExpiryBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []sweptReward{}
	for rows.Next() {
		var item sweptReward
		if err := rows.Scan(&item.id, &item.customerID, &item.previousStatus); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
