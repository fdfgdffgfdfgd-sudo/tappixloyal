package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

func queuePosterDaily(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `INSERT INTO integration_jobs(company_id,connection_id,job_type,resource,idempotency_key,payload)
		SELECT company_id,id,'poster_reconciliation','transactions','poster-daily:'||id||':'||current_date,jsonb_build_object('date',current_date)
		FROM integration_connections WHERE provider='poster' AND status IN('active','degraded') AND deleted_at IS NULL
		AND (last_sync_at IS NULL OR last_sync_at<now()-interval '20 hours') ON CONFLICT DO NOTHING`)
	return err
}

func processNextPosterJob(ctx context.Context, db *pgxpool.Pool, key []byte, client *http.Client) error {
	var jobID, companyID, connectionID, jobType, externalAccount string
	var encrypted []byte
	var config json.RawMessage
	err := db.QueryRow(ctx, `UPDATE integration_jobs SET status='processing',started_at=now(),attempts=attempts+1
		WHERE id=(SELECT j.id FROM integration_jobs j JOIN integration_connections c ON c.id=j.connection_id
			WHERE c.provider='poster' AND j.job_type LIKE 'poster_%' AND j.status IN('pending','failed') AND j.available_at<=now()
			ORDER BY j.created_at FOR UPDATE OF j SKIP LOCKED LIMIT 1)
		RETURNING id,company_id,connection_id,job_type`).Scan(&jobID, &companyID, &connectionID, &jobType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	err = db.QueryRow(ctx, `SELECT encrypted_credentials,config,coalesce(external_account_id,'') FROM integration_connections WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID, connectionID).Scan(&encrypted, &config, &externalAccount)
	if err != nil {
		return finishPosterJob(ctx, db, jobID, err)
	}
	plaintext, err := decryptIntegrationSecret(key, encrypted)
	if err != nil {
		return finishPosterJob(ctx, db, jobID, err)
	}
	credentials := map[string]string{}
	if err = json.Unmarshal(plaintext, &credentials); err != nil {
		return finishPosterJob(ctx, db, jobID, err)
	}
	connection := posintegration.Connection{ID: connectionID, CompanyID: companyID, Provider: "poster", ExternalAccountID: externalAccount, Config: config, Credentials: credentials}
	adapter := posintegration.NewPosterAdapter(client, envOr("POSTER_API_BASE_URL", "https://joinposter.com/api"))
	var result map[string]any
	switch jobType {
	case "poster_locations":
		result, err = importPosterLocations(ctx, db, adapter, connection)
	case "poster_customers":
		result, err = importPosterCustomers(ctx, db, adapter, connection)
	case "poster_transactions":
		result, err = importPosterTransactions(ctx, db, adapter, connection, time.Now().AddDate(0, 0, -90).Format("20060102"))
	case "poster_reconciliation":
		result, err = reconcilePoster(ctx, db, adapter, connection)
	case "poster_webhook_transaction":
		result, err = processPosterWebhookTransaction(ctx, db, adapter, connection, jobID)
	default:
		err = fmt.Errorf("unsupported Poster job type %s", jobType)
	}
	if err != nil {
		return finishPosterJob(ctx, db, jobID, err)
	}
	resultJSON, _ := json.Marshal(result)
	_, err = db.Exec(ctx, `UPDATE integration_jobs SET status='succeeded',completed_at=now(),last_error=NULL,result=$2 WHERE id=$1`, jobID, resultJSON)
	if err == nil {
		_, err = db.Exec(ctx, `UPDATE integration_connections SET status='active',last_connected_at=coalesce(last_connected_at,now()),last_sync_at=now(),last_error_code=NULL,last_error_message=NULL,updated_at=now() WHERE id=$1 AND company_id=$2`, connectionID, companyID)
	}
	return err
}

func processPosterWebhookTransaction(ctx context.Context, db *pgxpool.Pool, adapter *posintegration.PosterAdapter, connection posintegration.Connection, jobID string) (map[string]any, error) {
	var externalID, eventType, eventID, endpointID string
	err := db.QueryRow(ctx, `SELECT payload->>'externalId',payload->>'eventType',payload->>'eventId',payload->>'endpointId' FROM integration_jobs WHERE id=$1`, jobID).Scan(&externalID, &eventType, &eventID, &endpointID)
	if err != nil {
		return nil, err
	}
	transaction, err := adapter.GetTransaction(ctx, connection, externalID)
	if err != nil {
		return nil, err
	}
	service := posintegration.NewService(db)
	isRefund := transaction.Status == "cancelled" || strings.Contains(eventType, "return") || strings.Contains(eventType, "refund") || strings.Contains(eventType, "delete") || strings.Contains(eventType, "cancel")
	result := map[string]any{"externalId": externalID, "eventType": eventType}
	if isRefund {
		var originalID string
		var remaining float64
		err = db.QueryRow(ctx, `SELECT t.id,t.net_amount-coalesce((SELECT sum(r.net_amount) FROM sales_transactions r WHERE r.company_id=t.company_id AND r.original_transaction_id=t.id),0)
			FROM sales_transactions t WHERE t.company_id=$1 AND t.provider='poster' AND t.external_id=$2 AND t.original_transaction_id IS NULL`, connection.CompanyID, externalID).Scan(&originalID, &remaining)
		if errors.Is(err, pgx.ErrNoRows) {
			transaction.Status = "completed"
			transaction.Source = "poster_webhook_recovery"
			created, ingestErr := service.Ingest(ctx, transaction)
			if ingestErr != nil {
				return nil, ingestErr
			}
			originalID, remaining, err = created.TransactionID, transaction.NetAmount, nil
		}
		if err != nil {
			return nil, err
		}
		if remaining > 0 {
			refunded, refundErr := service.Refund(ctx, posintegration.RefundInput{CompanyID: connection.CompanyID, OriginalID: originalID, ExternalID: "return:" + externalID + ":" + eventID, Amount: remaining, Reason: "Возврат Poster"})
			if refundErr != nil {
				return nil, refundErr
			}
			result["refund"] = refunded
		} else {
			result["duplicate"] = true
		}
	} else {
		transaction.Status = "completed"
		transaction.Source = "poster_webhook"
		ingested, ingestErr := service.Ingest(ctx, transaction)
		if ingestErr != nil {
			return nil, ingestErr
		}
		result["transaction"] = ingested
	}
	_, err = db.Exec(ctx, `UPDATE webhook_deliveries SET status='succeeded',attempts=attempts+1,processed_at=now(),last_error=NULL WHERE endpoint_id=$1 AND event_id=$2 AND direction='inbound'`, endpointID, eventID)
	return result, err
}

func finishPosterJob(ctx context.Context, db *pgxpool.Pool, jobID string, cause error) error {
	_, err := db.Exec(ctx, `UPDATE integration_jobs SET status=CASE WHEN attempts>=max_attempts THEN 'dead' ELSE 'failed' END,last_error=$2,available_at=now()+make_interval(secs=>least(3600,power(2,attempts)::integer)) WHERE id=$1`, jobID, truncate(cause.Error(), 4000))
	if err == nil {
		_, err = db.Exec(ctx, `UPDATE webhook_deliveries d SET status=CASE WHEN j.attempts>=j.max_attempts THEN 'dead' ELSE 'failed' END,
			attempts=j.attempts,last_error=$2,processed_at=CASE WHEN j.attempts>=j.max_attempts THEN now() ELSE NULL END
			FROM integration_jobs j WHERE j.id=$1 AND j.job_type='poster_webhook_transaction' AND d.endpoint_id=(j.payload->>'endpointId')::uuid AND d.event_id=j.payload->>'eventId' AND d.direction='inbound'`, jobID, truncate(cause.Error(), 4000))
	}
	return errors.Join(cause, err)
}

func importPosterLocations(ctx context.Context, db *pgxpool.Pool, adapter *posintegration.PosterAdapter, connection posintegration.Connection) (map[string]any, error) {
	locations, err := adapter.ListLocations(ctx, connection)
	if err != nil {
		return nil, err
	}
	for _, location := range locations {
		_, err = db.Exec(ctx, `INSERT INTO integration_location_mappings(company_id,connection_id,external_location_id,external_location_name,status)
			VALUES($1,$2,$3,$4,'unmapped') ON CONFLICT(connection_id,external_location_id) DO UPDATE SET external_location_name=excluded.external_location_name,updated_at=now()`, connection.CompanyID, connection.ID, location.ExternalID, location.Name)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"imported": len(locations)}, nil
}

func importPosterCustomers(ctx context.Context, db *pgxpool.Pool, adapter *posintegration.PosterAdapter, connection posintegration.Connection) (map[string]any, error) {
	cursor, imported, linked, conflicts := "", 0, 0, 0
	for page := 0; page < 1000; page++ {
		batch, err := adapter.ImportCustomers(ctx, connection, cursor)
		if err != nil {
			return nil, err
		}
		for _, customer := range batch.Customers {
			imported++
			if customer.Phone == "" {
				_, err = db.Exec(ctx, `INSERT INTO integration_customer_links(company_id,connection_id,external_customer_id,status,match_method,metadata,last_synced_at)
					VALUES($1,$2,$3,'pending','external_id',jsonb_build_object('reason','missing_phone'),now()) ON CONFLICT(connection_id,external_customer_id) DO UPDATE SET status='pending',metadata=excluded.metadata,last_synced_at=now(),updated_at=now()`, connection.CompanyID, connection.ID, customer.ExternalID)
				conflicts++
				continue
			}
			var customerID string
			err = db.QueryRow(ctx, `INSERT INTO customers(company_id,first_name,last_name,phone) VALUES($1,$2,$3,$4)
				ON CONFLICT(company_id,phone) WHERE deleted_at IS NULL DO UPDATE SET updated_at=now() RETURNING id`, connection.CompanyID, customer.FirstName, customer.LastName, customer.Phone).Scan(&customerID)
			if err != nil {
				return nil, err
			}
			_, err = db.Exec(ctx, `INSERT INTO integration_customer_links(company_id,connection_id,customer_id,external_customer_id,normalized_phone,status,match_method,last_synced_at)
				VALUES($1,$2,$3,$4,$5,'linked','phone',now()) ON CONFLICT(connection_id,external_customer_id) DO UPDATE SET customer_id=excluded.customer_id,normalized_phone=excluded.normalized_phone,status='linked',match_method='phone',last_synced_at=now(),updated_at=now()`, connection.CompanyID, connection.ID, customerID, customer.ExternalID, customer.Phone)
			if err != nil {
				return nil, err
			}
			linked++
		}
		if batch.NextCursor == "" {
			break
		}
		cursor = batch.NextCursor
	}
	return map[string]any{"imported": imported, "linked": linked, "pending": conflicts}, nil
}

func importPosterTransactions(ctx context.Context, db *pgxpool.Pool, adapter *posintegration.PosterAdapter, connection posintegration.Connection, cursor string) (map[string]any, error) {
	batch, err := adapter.ImportTransactions(ctx, connection, cursor)
	if err != nil {
		return nil, err
	}
	service := posintegration.NewService(db)
	imported, duplicates, skipped := 0, 0, 0
	for _, transaction := range batch.Transactions {
		if transaction.ExternalID == "<nil>" || transaction.ExternalID == "" || transaction.OccurredAt.IsZero() {
			skipped++
			continue
		}
		result, ingestErr := service.Ingest(ctx, transaction)
		if ingestErr != nil {
			return nil, fmt.Errorf("Poster transaction %s: %w", transaction.ExternalID, ingestErr)
		}
		if result.Duplicate {
			duplicates++
		} else {
			imported++
		}
	}
	_, _ = db.Exec(ctx, `INSERT INTO integration_sync_cursors(company_id,connection_id,resource,cursor_value,watermark_at,last_success_at,last_attempt_at)
		VALUES($1,$2,'transactions',$3,now(),now(),now()) ON CONFLICT(connection_id,resource) DO UPDATE SET cursor_value=excluded.cursor_value,watermark_at=now(),last_success_at=now(),last_attempt_at=now(),updated_at=now()`, connection.CompanyID, connection.ID, batch.NextCursor)
	return map[string]any{"received": len(batch.Transactions), "imported": imported, "duplicates": duplicates, "skipped": skipped}, nil
}

func reconcilePoster(ctx context.Context, db *pgxpool.Pool, adapter *posintegration.PosterAdapter, connection posintegration.Connection) (map[string]any, error) {
	start := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	var runID string
	err := db.QueryRow(ctx, `INSERT INTO reconciliation_runs(company_id,connection_id,resource,status,range_start,range_end,started_at) VALUES($1,$2,'transactions','processing',$3,$4,now()) RETURNING id`, connection.CompanyID, connection.ID, start, time.Now()).Scan(&runID)
	if err != nil {
		return nil, err
	}
	result, syncErr := importPosterTransactions(ctx, db, adapter, connection, start.Format("20060102"))
	if syncErr != nil {
		_, _ = db.Exec(ctx, `UPDATE reconciliation_runs SET status='failed',last_error=$2,completed_at=now() WHERE id=$1`, runID, truncate(syncErr.Error(), 4000))
		return nil, syncErr
	}
	received, _ := result["received"].(int)
	duplicates, _ := result["duplicates"].(int)
	imported, _ := result["imported"].(int)
	_, err = db.Exec(ctx, `UPDATE reconciliation_runs SET status='succeeded',provider_count=$2,local_count=$3,missing_count=$4,repaired_count=$4,details=$5,completed_at=now() WHERE id=$1`, runID, received, duplicates+imported, imported, mustJSON(result))
	return map[string]any{"runId": runID, "providerCount": received, "repaired": imported}, err
}

func mustJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}
