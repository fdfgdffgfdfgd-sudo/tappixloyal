package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func StartIntegrationWorkers(ctx context.Context, db *pgxpool.Pool, encryptionSecret string) {
	key := integrationEncryptionKey(encryptionSecret)
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		daily := time.NewTicker(time.Hour)
		defer ticker.Stop()
		defer daily.Stop()
		_ = queuePosterDaily(ctx, db)
		for {
			if err := fanoutOutbox(ctx, db); err != nil && ctx.Err() == nil {
				slog.Warn("integration outbox fanout failed", "error", err)
			}
			if err := deliverNextWebhook(ctx, db, key, client); err != nil && ctx.Err() == nil {
				slog.Warn("outbound webhook delivery failed", "error", err)
			}
			if err := processNextPosterJob(ctx, db, key, client); err != nil && ctx.Err() == nil {
				slog.Warn("Poster synchronization failed", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-daily.C:
				_ = queuePosterDaily(ctx, db)
			case <-ticker.C:
			}
		}
	}()
}

func fanoutOutbox(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,company_id,event_type,payload FROM outbox_events
		WHERE status IN ('pending','failed') AND available_at<=now() AND company_id IS NOT NULL
		ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 50`)
	if err != nil {
		return err
	}
	type event struct {
		id, companyID, eventType string
		payload                  json.RawMessage
	}
	events := []event{}
	for rows.Next() {
		var item event
		if err = rows.Scan(&item.id, &item.companyID, &item.eventType, &item.payload); err != nil {
			rows.Close()
			return err
		}
		events = append(events, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range events {
		_, err = tx.Exec(ctx, `INSERT INTO webhook_deliveries(company_id,endpoint_id,outbox_event_id,event_type,event_id,direction,payload)
			SELECT $1,id,$2,$3,$2::text,'outbound',$4 FROM webhook_endpoints
			WHERE company_id=$1 AND direction='outbound' AND status='active' AND deleted_at IS NULL
			AND (cardinality(event_types)=0 OR $3=ANY(event_types)) ON CONFLICT DO NOTHING`, item.companyID, item.id, item.eventType, item.payload)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE outbox_events SET status='sent',attempts=attempts+1,processed_at=now(),last_error=NULL WHERE id=$1`, item.id)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func deliverNextWebhook(ctx context.Context, db *pgxpool.Pool, key []byte, client *http.Client) error {
	var deliveryID, companyID, endpointID, targetURL, eventType, eventID string
	var payload json.RawMessage
	var encrypted []byte
	var attempts, maxAttempts int
	err := db.QueryRow(ctx, `UPDATE webhook_deliveries d SET status='processing'
		FROM webhook_endpoints e WHERE d.id=(SELECT d2.id FROM webhook_deliveries d2 JOIN webhook_endpoints e2 ON e2.id=d2.endpoint_id
		WHERE d2.direction='outbound' AND d2.status IN ('pending','failed') AND d2.next_attempt_at<=now() AND e2.status='active'
		ORDER BY d2.next_attempt_at FOR UPDATE OF d2 SKIP LOCKED LIMIT 1)
		AND e.id=d.endpoint_id
		RETURNING d.id,d.company_id,d.endpoint_id,e.url,e.encrypted_secret,d.event_type,d.event_id,d.payload,d.attempts,d.max_attempts`).Scan(
		&deliveryID, &companyID, &endpointID, &targetURL, &encrypted, &eventType, &eventID, &payload, &attempts, &maxAttempts)
	if err != nil {
		return nil
	}
	secret, err := decryptIntegrationSecret(key, encrypted)
	if err != nil {
		return failWebhookDelivery(ctx, db, deliveryID, companyID, endpointID, attempts, maxAttempts, 0, "secret decryption failed")
	}
	if err = validateOutboundURL(ctx, targetURL); err != nil {
		return failWebhookDelivery(ctx, db, deliveryID, companyID, endpointID, attempts, maxAttempts, 0, err.Error())
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return failWebhookDelivery(ctx, db, deliveryID, companyID, endpointID, attempts, maxAttempts, 0, err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Tappix-Webhooks/1.0")
	req.Header.Set("X-Tappix-Event", eventType)
	req.Header.Set("X-Tappix-Event-ID", eventID)
	req.Header.Set("X-Tappix-Timestamp", timestamp)
	req.Header.Set("X-Tappix-Signature", "sha256="+signature)
	response, err := client.Do(req)
	if err != nil {
		return failWebhookDelivery(ctx, db, deliveryID, companyID, endpointID, attempts, maxAttempts, 0, err.Error())
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return failWebhookDelivery(ctx, db, deliveryID, companyID, endpointID, attempts, maxAttempts, response.StatusCode, string(responseBody))
	}
	_, err = db.Exec(ctx, `UPDATE webhook_deliveries SET status='succeeded',attempts=attempts+1,response_status=$2,response_body=$3,processed_at=now(),last_error=NULL WHERE id=$1 AND company_id=$4`, deliveryID, response.StatusCode, string(responseBody), companyID)
	if err == nil {
		_, err = db.Exec(ctx, `UPDATE webhook_endpoints SET failure_count=0,last_delivery_at=now(),last_success_at=now(),last_error=NULL,updated_at=now() WHERE id=$1 AND company_id=$2`, endpointID, companyID)
	}
	return err
}

func failWebhookDelivery(ctx context.Context, db *pgxpool.Pool, deliveryID, companyID, endpointID string, attempts, maxAttempts, responseStatus int, message string) error {
	nextAttempts := attempts + 1
	status := "failed"
	if nextAttempts >= maxAttempts {
		status = "dead"
	}
	_, err := db.Exec(ctx, `UPDATE webhook_deliveries SET status=$2,attempts=$3,response_status=nullif($4,0),last_error=$5,
		next_attempt_at=now()+make_interval(secs=>least(3600,power(2,$3)::integer)),processed_at=CASE WHEN $2='dead' THEN now() ELSE NULL END
		WHERE id=$1 AND company_id=$6`, deliveryID, status, nextAttempts, responseStatus, truncate(message, 4000), companyID)
	if err == nil {
		_, err = db.Exec(ctx, `UPDATE webhook_endpoints SET failure_count=failure_count+1,last_delivery_at=now(),last_error=$3,status=CASE WHEN failure_count+1>=20 THEN 'error' ELSE status END,updated_at=now() WHERE id=$1 AND company_id=$2`, endpointID, companyID, truncate(message, 4000))
	}
	return err
}
