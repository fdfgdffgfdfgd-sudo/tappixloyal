package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type locationMappingInput struct {
	BranchID string `json:"branchId"`
	Status   string `json:"status"`
}

type customerLinkInput struct {
	CustomerID string `json:"customerId"`
	Status     string `json:"status"`
}

func posterString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok && value != nil {
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(toString(value)), ".0"), "<nil>"))
			if text != "" {
				return text
			}
		}
	}
	if nested, ok := payload["data"].(map[string]any); ok {
		return posterString(nested, keys...)
	}
	return ""
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		encoded, _ := json.Marshal(value)
		return strings.Trim(string(encoded), `"`)
	}
}

func (a *api) posterWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, integrationWebhookMaxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 413, "PAYLOAD_TOO_LARGE", "Webhook превышает допустимый размер")
		return
	}
	var endpointID, tenant, connectionID string
	var encrypted []byte
	err = a.db.QueryRow(r.Context(), `SELECT w.id,w.company_id,w.connection_id,w.encrypted_secret FROM webhook_endpoints w JOIN integration_connections c ON c.id=w.connection_id
		WHERE w.inbound_key=$1 AND w.direction='inbound' AND w.status='active' AND w.deleted_at IS NULL AND c.provider='poster' AND c.deleted_at IS NULL`, r.PathValue("key")).Scan(&endpointID, &tenant, &connectionID, &encrypted)
	if err != nil {
		fail(w, 404, "WEBHOOK_NOT_FOUND", "Poster webhook не найден")
		return
	}
	secret, err := decryptIntegrationSecret(a.integrationKey, encrypted)
	if err != nil {
		fail(w, 500, "WEBHOOK_SECRET_ERROR", "Poster webhook недоступен")
		return
	}
	provided := r.URL.Query().Get("secret")
	if provided == "" {
		provided = r.Header.Get("X-Poster-Secret")
	}
	if len(provided) != len(secret) || subtle.ConstantTimeCompare([]byte(provided), secret) != 1 {
		fail(w, 401, "WEBHOOK_SECRET_INVALID", "Некорректный секрет Poster webhook")
		return
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	payload := map[string]any{}
	if err = decoder.Decode(&payload); err != nil {
		fail(w, 422, "WEBHOOK_PAYLOAD_INVALID", "Некорректный Poster payload")
		return
	}
	eventType := strings.ToLower(posterString(payload, "event", "type", "action"))
	externalID := posterString(payload, "transaction_id", "transactionId", "object_id", "objectId")
	if externalID == "" || (!strings.Contains(eventType, "transact") && !strings.Contains(eventType, "close") && !strings.Contains(eventType, "return") && !strings.Contains(eventType, "refund") && !strings.Contains(eventType, "delete")) {
		fail(w, 422, "POSTER_EVENT_UNSUPPORTED", "Webhook не содержит поддерживаемое событие чека")
		return
	}
	digest := sha256.Sum256(body)
	eventID := posterString(payload, "event_id", "eventId", "id")
	if eventID == "" {
		eventID = hex.EncodeToString(digest[:])
	}
	result, err := a.db.Exec(r.Context(), `WITH delivery AS (
		INSERT INTO webhook_deliveries(company_id,endpoint_id,event_type,event_id,direction,status,request_headers,payload,received_at)
		VALUES($1,$2,$3,$4,'inbound','pending',jsonb_build_object('userAgent',$5),$6,now()) ON CONFLICT(endpoint_id,event_id,direction) DO NOTHING RETURNING id
	) INSERT INTO integration_jobs(company_id,connection_id,job_type,resource,idempotency_key,payload)
		SELECT $1,$7,'poster_webhook_transaction','transactions','poster-webhook:'||$2||':'||$4,jsonb_build_object('endpointId',$2::text,'eventId',$4::text,'eventType',$3::text,'externalId',$8::text) FROM delivery`, tenant, endpointID, eventType, eventID, r.UserAgent(), body, connectionID, externalID)
	if err != nil {
		fail(w, 500, "POSTER_WEBHOOK_QUEUE_FAILED", "Не удалось поставить Poster webhook в очередь")
		return
	}
	write(w, 202, envelope{Success: true, Data: map[string]any{"accepted": true, "duplicate": result.RowsAffected() == 0, "eventId": eventID}})
}

func (a *api) syncIntegrationConnection(w http.ResponseWriter, r *http.Request) {
	var provider string
	err := a.db.QueryRow(r.Context(), `SELECT provider FROM integration_connections WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), r.PathValue("id")).Scan(&provider)
	if err != nil {
		fail(w, 404, "INTEGRATION_NOT_FOUND", "Подключение не найдено")
		return
	}
	if provider != "poster" {
		fail(w, 422, "SYNC_NOT_SUPPORTED", "Read-only импорт сейчас доступен только для Poster")
		return
	}
	batch := time.Now().UTC().Format("20060102T150405.000000000")
	for _, jobType := range []string{"poster_locations", "poster_customers", "poster_transactions"} {
		_, err = a.db.Exec(r.Context(), `INSERT INTO integration_jobs(company_id,connection_id,job_type,resource,idempotency_key,payload)
			VALUES($1,$2,$3,replace($3,'poster_',''),$4,jsonb_build_object('manual',true))`, companyID(r), r.PathValue("id"), jobType, "poster-sync:"+r.PathValue("id")+":"+batch+":"+jobType)
		if err != nil {
			fail(w, 500, "SYNC_QUEUE_FAILED", "Не удалось запустить синхронизацию")
			return
		}
	}
	write(w, 202, envelope{Success: true, Data: map[string]any{"connectionId": r.PathValue("id"), "status": "pending", "resources": []string{"locations", "customers", "transactions"}}})
}

func (a *api) integrationLocationMappings(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT m.id,m.external_location_id,m.external_location_name,m.status,coalesce(m.branch_id::text,''),coalesce(b.name,''),m.updated_at
		FROM integration_location_mappings m LEFT JOIN branches b ON b.id=m.branch_id AND b.company_id=m.company_id
		WHERE m.company_id=$1 AND m.connection_id=$2 ORDER BY m.external_location_name`, companyID(r), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "LOCATION_MAPPINGS_FAILED", "Не удалось загрузить сопоставления филиалов")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, externalID, name, status, branchID, branchName string
		var updated time.Time
		if err := rows.Scan(&id, &externalID, &name, &status, &branchID, &branchName, &updated); err != nil {
			fail(w, 500, "LOCATION_MAPPINGS_FAILED", "Не удалось загрузить сопоставления филиалов")
			return
		}
		items = append(items, map[string]any{"id": id, "externalLocationId": externalID, "externalLocationName": name, "status": status, "branchId": branchID, "branchName": branchName, "updatedAt": updated})
	}
	if rows.Err() != nil {
		fail(w, 500, "LOCATION_MAPPINGS_FAILED", "Не удалось загрузить сопоставления филиалов")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) updateIntegrationLocationMapping(w http.ResponseWriter, r *http.Request) {
	var in locationMappingInput
	if !decode(w, r, &in) {
		return
	}
	in.Status = strings.ToLower(strings.TrimSpace(in.Status))
	if in.Status == "" {
		in.Status = "mapped"
	}
	if in.Status != "mapped" && in.Status != "ignored" && in.Status != "disabled" && in.Status != "unmapped" {
		fail(w, 422, "INVALID_MAPPING_STATUS", "Некорректный статус сопоставления")
		return
	}
	if in.Status == "mapped" && in.BranchID == "" {
		fail(w, 422, "BRANCH_REQUIRED", "Для сопоставления выберите филиал")
		return
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE integration_location_mappings m SET branch_id=CASE WHEN $5='mapped' THEN nullif($4,'')::uuid ELSE NULL END,status=$5,updated_at=now()
		WHERE m.company_id=$1 AND m.connection_id=$2 AND m.id=$3
		AND ($5<>'mapped' OR EXISTS(SELECT 1 FROM branches b WHERE b.company_id=$1 AND b.id=nullif($4,'')::uuid AND b.deleted_at IS NULL))`, companyID(r), r.PathValue("id"), r.PathValue("mappingId"), in.BranchID, in.Status)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "MAPPING_OR_BRANCH_NOT_FOUND", "Сопоставление или филиал не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"updated": true, "status": in.Status, "branchId": in.BranchID}})
}

func (a *api) integrationCustomerLinks(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	rows, err := a.db.Query(r.Context(), `SELECT l.id,l.external_customer_id,coalesce(l.normalized_phone,''),l.status,l.match_method,l.metadata,
		coalesce(l.customer_id::text,''),coalesce(c.first_name||' '||c.last_name,''),coalesce(c.phone,''),l.updated_at
		FROM integration_customer_links l LEFT JOIN customers c ON c.id=l.customer_id AND c.company_id=l.company_id
		WHERE l.company_id=$1 AND l.connection_id=$2 AND ($3='' OR l.status=$3)
		ORDER BY CASE WHEN l.status IN('pending','conflict') THEN 0 ELSE 1 END,l.updated_at DESC LIMIT 100`, companyID(r), r.PathValue("id"), status)
	if err != nil {
		fail(w, 500, "CUSTOMER_LINKS_FAILED", "Не удалось загрузить связи клиентов")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, externalID, phone, linkStatus, method, customerID, customerName, customerPhone string
		var metadata json.RawMessage
		var updated time.Time
		if err := rows.Scan(&id, &externalID, &phone, &linkStatus, &method, &metadata, &customerID, &customerName, &customerPhone, &updated); err != nil {
			fail(w, 500, "CUSTOMER_LINKS_FAILED", "Не удалось загрузить связи клиентов")
			return
		}
		items = append(items, map[string]any{"id": id, "externalCustomerId": externalID, "normalizedPhone": phone, "status": linkStatus, "matchMethod": method, "metadata": metadata, "customerId": customerID, "customerName": strings.TrimSpace(customerName), "customerPhone": customerPhone, "updatedAt": updated})
	}
	if rows.Err() != nil {
		fail(w, 500, "CUSTOMER_LINKS_FAILED", "Не удалось загрузить связи клиентов")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) updateIntegrationCustomerLink(w http.ResponseWriter, r *http.Request) {
	var in customerLinkInput
	if !decode(w, r, &in) {
		return
	}
	in.Status = strings.ToLower(strings.TrimSpace(in.Status))
	if in.Status != "linked" && in.Status != "ignored" && in.Status != "pending" {
		fail(w, 422, "INVALID_LINK_STATUS", "Выберите клиента или исключите запись")
		return
	}
	if in.Status == "linked" && strings.TrimSpace(in.CustomerID) == "" {
		fail(w, 422, "CUSTOMER_REQUIRED", "Для связи выберите клиента Tappix")
		return
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE integration_customer_links l SET customer_id=CASE WHEN $5='linked' THEN nullif($4,'')::uuid ELSE NULL END,
		status=$5,match_method=CASE WHEN $5='linked' THEN 'manual' ELSE match_method END,updated_at=now()
		WHERE l.company_id=$1 AND l.connection_id=$2 AND l.id=$3
		AND ($5<>'linked' OR EXISTS(SELECT 1 FROM customers c WHERE c.company_id=$1 AND c.id=nullif($4,'')::uuid AND c.deleted_at IS NULL))`, companyID(r), r.PathValue("id"), r.PathValue("linkId"), strings.TrimSpace(in.CustomerID), in.Status)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "LINK_OR_CUSTOMER_NOT_FOUND", "Связь или клиент не найден")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"updated": true, "status": in.Status, "customerId": in.CustomerID}})
}

func (a *api) integrationSyncStatus(w http.ResponseWriter, r *http.Request) {
	var provider, status string
	var lastSync *time.Time
	if err := a.db.QueryRow(r.Context(), `SELECT provider,status,last_sync_at FROM integration_connections WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), r.PathValue("id")).Scan(&provider, &status, &lastSync); err != nil {
		fail(w, 404, "INTEGRATION_NOT_FOUND", "Подключение не найдено")
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT id,job_type,resource,status,attempts,max_attempts,created_at,started_at,completed_at,coalesce(last_error,''),result FROM integration_jobs WHERE company_id=$1 AND connection_id=$2 ORDER BY created_at DESC LIMIT 30`, companyID(r), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "SYNC_STATUS_FAILED", "Не удалось загрузить состояние синхронизации")
		return
	}
	defer rows.Close()
	jobs := []map[string]any{}
	for rows.Next() {
		var id, jobType, resource, jobStatus, lastError string
		var attempts, maxAttempts int
		var created time.Time
		var started, completed *time.Time
		var result json.RawMessage
		if err := rows.Scan(&id, &jobType, &resource, &jobStatus, &attempts, &maxAttempts, &created, &started, &completed, &lastError, &result); err != nil {
			fail(w, 500, "SYNC_STATUS_FAILED", "Не удалось загрузить состояние синхронизации")
			return
		}
		jobs = append(jobs, map[string]any{"id": id, "jobType": jobType, "resource": resource, "status": jobStatus, "attempts": attempts, "maxAttempts": maxAttempts, "createdAt": created, "startedAt": started, "completedAt": completed, "lastError": lastError, "result": result})
	}
	if rows.Err() != nil {
		fail(w, 500, "SYNC_STATUS_FAILED", "Не удалось загрузить состояние синхронизации")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"connectionId": r.PathValue("id"), "provider": provider, "status": status, "lastSyncAt": lastSync, "jobs": jobs}})
}

func (a *api) integrationReconciliations(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id,resource,status,range_start,range_end,provider_count,local_count,missing_count,mismatch_count,repaired_count,coalesce(last_error,''),created_at,completed_at
		FROM reconciliation_runs WHERE company_id=$1 AND connection_id=$2 ORDER BY created_at DESC LIMIT 20`, companyID(r), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "RECONCILIATIONS_FAILED", "Не удалось загрузить историю сверок")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, resource, status, lastError string
		var rangeStart, rangeEnd, created time.Time
		var completed *time.Time
		var providerCount, localCount, missingCount, mismatchCount, repairedCount int
		if err := rows.Scan(&id, &resource, &status, &rangeStart, &rangeEnd, &providerCount, &localCount, &missingCount, &mismatchCount, &repairedCount, &lastError, &created, &completed); err != nil {
			fail(w, 500, "RECONCILIATIONS_FAILED", "Не удалось загрузить историю сверок")
			return
		}
		items = append(items, map[string]any{"id": id, "resource": resource, "status": status, "rangeStart": rangeStart, "rangeEnd": rangeEnd, "providerCount": providerCount, "localCount": localCount, "missingCount": missingCount, "mismatchCount": mismatchCount, "repairedCount": repairedCount, "lastError": lastError, "createdAt": created, "completedAt": completed})
	}
	if rows.Err() != nil {
		fail(w, 500, "RECONCILIATIONS_FAILED", "Не удалось загрузить историю сверок")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) startIntegrationReconciliation(w http.ResponseWriter, r *http.Request) {
	var provider string
	if err := a.db.QueryRow(r.Context(), `SELECT provider FROM integration_connections WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL AND status<>'disabled'`, companyID(r), r.PathValue("id")).Scan(&provider); err != nil {
		fail(w, 404, "INTEGRATION_NOT_FOUND", "Подключение не найдено")
		return
	}
	if provider != "poster" {
		fail(w, 422, "RECONCILIATION_NOT_SUPPORTED", "Сверка сейчас доступна только для Poster")
		return
	}
	batch := time.Now().UTC().Format("20060102T150405.000000000")
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO integration_jobs(company_id,connection_id,job_type,resource,idempotency_key,payload)
		VALUES($1,$2,'poster_reconciliation','transactions',$3,jsonb_build_object('manual',true)) RETURNING id`, companyID(r), r.PathValue("id"), "poster-manual-reconciliation:"+r.PathValue("id")+":"+batch).Scan(&id)
	if err != nil {
		fail(w, 500, "RECONCILIATION_QUEUE_FAILED", "Не удалось запустить сверку")
		return
	}
	write(w, 202, envelope{Success: true, Data: map[string]any{"jobId": id, "status": "pending"}})
}
