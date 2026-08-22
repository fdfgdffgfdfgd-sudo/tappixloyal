package httpapi

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

const integrationWebhookMaxBody = 2 << 20

type integrationConnectionInput struct {
	Provider          string         `json:"provider"`
	Name              string         `json:"name"`
	AuthType          string         `json:"authType"`
	ExternalAccountID string         `json:"externalAccountId"`
	Credentials       map[string]any `json:"credentials"`
	Config            map[string]any `json:"config"`
}

type outboundWebhookInput struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	EventTypes []string `json:"eventTypes"`
}

func integrationEncryptionKey(secret string) []byte {
	configured := strings.TrimSpace(envOr("INTEGRATION_ENCRYPTION_KEY", ""))
	if configured != "" {
		secret = configured
	}
	sum := sha256.Sum256([]byte("tappix:integration:" + secret))
	return sum[:]
}

func encryptIntegrationSecret(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptIntegrationSecret(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("invalid encrypted secret")
	}
	nonce, payload := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, payload, nil)
}

func randomHex(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (a *api) listIntegrationConnections(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id,provider,name,status,auth_type,coalesce(external_account_id,''),capabilities,last_connected_at,last_sync_at,coalesce(last_error_code,''),coalesce(last_error_message,''),created_at
		FROM integration_connections WHERE company_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить подключения")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, provider, name, status, authType, account, errorCode, errorMessage string
		var capabilities json.RawMessage
		var connectedAt, syncAt *time.Time
		var createdAt time.Time
		if err = rows.Scan(&id, &provider, &name, &status, &authType, &account, &capabilities, &connectedAt, &syncAt, &errorCode, &errorMessage, &createdAt); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось прочитать подключения")
			return
		}
		items = append(items, map[string]any{"id": id, "provider": provider, "name": name, "status": status, "authType": authType, "externalAccountId": account, "capabilities": capabilities, "lastConnectedAt": connectedAt, "lastSyncAt": syncAt, "lastErrorCode": errorCode, "lastErrorMessage": errorMessage, "createdAt": createdAt})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось прочитать подключения")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) createIntegrationConnection(w http.ResponseWriter, r *http.Request) {
	if ok, limit := a.checkLimit(r.Context(), companyID(r), "integrations"); !ok {
		fail(w, 409, "PLAN_UPGRADE_REQUIRED", limitMessage("интеграций", limit))
		return
	}
	var in integrationConnectionInput
	if !decode(w, r, &in) {
		return
	}
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Name = strings.TrimSpace(in.Name)
	if in.Provider == "" || in.Name == "" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите провайдера и название подключения")
		return
	}
	if in.AuthType == "" {
		in.AuthType = "api_key"
	}
	credentials, err := json.Marshal(in.Credentials)
	if err != nil {
		fail(w, 422, "VALIDATION_ERROR", "Некорректные credentials")
		return
	}
	encrypted, err := encryptIntegrationSecret(a.integrationKey, credentials)
	if err != nil {
		fail(w, 500, "ENCRYPTION_ERROR", "Не удалось защитить credentials")
		return
	}
	config, _ := json.Marshal(in.Config)
	status := "draft"
	capabilities := []string{}
	if in.Provider == "poster" {
		plainCredentials := map[string]string{}
		for key, value := range in.Credentials {
			plainCredentials[key] = strings.TrimSpace(fmt.Sprint(value))
		}
		adapter := posintegration.NewPosterAdapter(&http.Client{Timeout: 20 * time.Second}, envOr("POSTER_API_BASE_URL", "https://joinposter.com/api"))
		locations, connectionErr := adapter.ListLocations(r.Context(), posintegration.Connection{Provider: "poster", Credentials: plainCredentials})
		if connectionErr != nil {
			fail(w, 422, "PROVIDER_AUTH_FAILED", "Poster не подтвердил подключение. Проверьте access token")
			return
		}
		status = "active"
		capabilities = []string{"locations", "customers", "transactions", "reconciliation"}
		if in.ExternalAccountID == "" && len(locations) > 0 {
			in.ExternalAccountID = locations[0].ExternalID
		}
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var id string
	err = a.db.QueryRow(r.Context(), `INSERT INTO integration_connections(company_id,provider,name,status,auth_type,encrypted_credentials,config,external_account_id,capabilities,created_by,last_connected_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,nullif($8,''),$9,$10,CASE WHEN $4='active' THEN now() END) RETURNING id`, companyID(r), in.Provider, in.Name, status, in.AuthType, encrypted, config, in.ExternalAccountID, capabilities, claims.Subject).Scan(&id)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось создать подключение")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "provider": in.Provider, "name": in.Name, "status": status, "externalAccountId": in.ExternalAccountID, "capabilities": capabilities}})
}

func (a *api) createInboundWebhook(w http.ResponseWriter, r *http.Request) {
	connectionID := r.PathValue("id")
	var provider string
	if err := a.db.QueryRow(r.Context(), `SELECT provider FROM integration_connections WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID(r), connectionID).Scan(&provider); err != nil {
		fail(w, 404, "INTEGRATION_NOT_FOUND", "Подключение не найдено")
		return
	}
	inboundKey, err := randomHex(20)
	if err != nil {
		fail(w, 500, "RANDOM_ERROR", "Не удалось создать webhook")
		return
	}
	secret, err := randomHex(32)
	if err != nil {
		fail(w, 500, "RANDOM_ERROR", "Не удалось создать webhook")
		return
	}
	encrypted, err := encryptIntegrationSecret(a.integrationKey, []byte(secret))
	if err != nil {
		fail(w, 500, "ENCRYPTION_ERROR", "Не удалось защитить webhook secret")
		return
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var id string
	err = a.db.QueryRow(r.Context(), `INSERT INTO webhook_endpoints(company_id,connection_id,direction,name,inbound_key,encrypted_secret,secret_prefix,event_types,created_by)
		VALUES($1,$2,'inbound',$3,$4,$5,$6,ARRAY['transaction.completed','transaction.refunded','transaction.cancelled'],$7) RETURNING id`, companyID(r), connectionID, provider+" inbound", inboundKey, encrypted, secret[:8], claims.Subject).Scan(&id)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось создать webhook")
		return
	}
	callbackURL := envOr("APP_URL", "http://localhost:8080") + "/api/v1/integrations/inbound/" + inboundKey
	if provider == "poster" {
		callbackURL = envOr("APP_URL", "http://localhost:8080") + "/api/v1/integrations/poster/" + inboundKey + "?secret=" + url.QueryEscape(secret)
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "url": callbackURL, "secret": secret, "secretPrefix": secret[:8], "notice": "Секрет показывается только один раз"}})
}

func (a *api) getInboundWebhook(w http.ResponseWriter, r *http.Request) {
	var id, name, status, secretPrefix string
	var eventTypes []string
	var failureCount int
	var lastDelivery, lastSuccess *time.Time
	err := a.db.QueryRow(r.Context(), `SELECT id,name,status,secret_prefix,event_types,failure_count,last_delivery_at,last_success_at
		FROM webhook_endpoints WHERE company_id=$1 AND connection_id=$2 AND direction='inbound' AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1`, companyID(r), r.PathValue("id")).Scan(&id, &name, &status, &secretPrefix, &eventTypes, &failureCount, &lastDelivery, &lastSuccess)
	if errors.Is(err, pgx.ErrNoRows) {
		write(w, 200, envelope{Success: true, Data: nil})
		return
	}
	if err != nil {
		fail(w, 500, "WEBHOOK_STATUS_FAILED", "Не удалось загрузить состояние webhook")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": id, "name": name, "status": status, "secretPrefix": secretPrefix, "eventTypes": eventTypes, "failureCount": failureCount, "lastDeliveryAt": lastDelivery, "lastSuccessAt": lastSuccess}})
}

func validateOutboundURL(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return errors.New("outbound webhook must use a public HTTPS URL")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, parsed.Hostname())
	if err != nil || len(addresses) == 0 {
		return errors.New("outbound webhook host cannot be resolved")
	}
	for _, address := range addresses {
		ip := address.IP
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return errors.New("outbound webhook cannot target a private address")
		}
	}
	return nil
}

func (a *api) createOutboundWebhook(w http.ResponseWriter, r *http.Request) {
	var in outboundWebhookInput
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || validateOutboundURL(r.Context(), in.URL) != nil {
		fail(w, 422, "VALIDATION_ERROR", "Укажите название и публичный HTTPS URL")
		return
	}
	secret, err := randomHex(32)
	if err != nil {
		fail(w, 500, "RANDOM_ERROR", "Не удалось создать webhook")
		return
	}
	encrypted, err := encryptIntegrationSecret(a.integrationKey, []byte(secret))
	if err != nil {
		fail(w, 500, "ENCRYPTION_ERROR", "Не удалось защитить webhook secret")
		return
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var id string
	err = a.db.QueryRow(r.Context(), `INSERT INTO webhook_endpoints(company_id,direction,name,url,encrypted_secret,secret_prefix,event_types,created_by)
		VALUES($1,'outbound',$2,$3,$4,$5,$6,$7) RETURNING id`, companyID(r), in.Name, in.URL, encrypted, secret[:8], in.EventTypes, claims.Subject).Scan(&id)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось создать webhook")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "name": in.Name, "url": in.URL, "eventTypes": in.EventTypes, "secret": secret, "secretPrefix": secret[:8], "notice": "Секрет показывается только один раз"}})
}

func (a *api) integrationInboundWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, integrationWebhookMaxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 413, "PAYLOAD_TOO_LARGE", "Webhook превышает допустимый размер")
		return
	}
	var endpointID, tenant, connectionID, provider string
	var encrypted []byte
	err = a.db.QueryRow(r.Context(), `SELECT w.id,w.company_id,w.connection_id,c.provider,w.encrypted_secret FROM webhook_endpoints w JOIN integration_connections c ON c.id=w.connection_id AND c.company_id=w.company_id WHERE w.inbound_key=$1 AND w.direction='inbound' AND w.status='active' AND w.deleted_at IS NULL`, r.PathValue("key")).Scan(&endpointID, &tenant, &connectionID, &provider, &encrypted)
	if err != nil {
		fail(w, 404, "WEBHOOK_NOT_FOUND", "Webhook не найден")
		return
	}
	secret, err := decryptIntegrationSecret(a.integrationKey, encrypted)
	if err != nil {
		fail(w, 500, "WEBHOOK_SECRET_ERROR", "Webhook недоступен")
		return
	}
	timestamp := r.Header.Get("X-Tappix-Timestamp")
	signature := strings.TrimPrefix(r.Header.Get("X-Tappix-Signature"), "sha256=")
	seconds, parseErr := strconv.ParseInt(timestamp, 10, 64)
	if parseErr != nil || time.Since(time.Unix(seconds, 0)) > 5*time.Minute || time.Until(time.Unix(seconds, 0)) > time.Minute {
		fail(w, 401, "WEBHOOK_TIMESTAMP_INVALID", "Webhook timestamp недействителен")
		return
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(signature))) {
		fail(w, 401, "WEBHOOK_SIGNATURE_INVALID", "Подпись webhook недействительна")
		return
	}
	var transaction posintegration.CanonicalTransaction
	if err = json.Unmarshal(body, &transaction); err != nil {
		fail(w, 422, "WEBHOOK_PAYLOAD_INVALID", "Некорректный payload webhook")
		return
	}
	transaction.CompanyID = tenant
	transaction.ConnectionID = connectionID
	if transaction.Provider == "" {
		transaction.Provider = provider
	}
	if transaction.Provider != provider {
		fail(w, 422, "PROVIDER_MISMATCH", "Провайдер payload не совпадает с подключением")
		return
	}
	eventID := transaction.Provider + ":" + transaction.ExternalID + ":" + transaction.Status
	var deliveryID string
	err = a.db.QueryRow(r.Context(), `INSERT INTO webhook_deliveries(company_id,endpoint_id,event_type,event_id,direction,status,request_headers,payload,received_at)
		VALUES($1,$2,$3,$4,'inbound','processing',jsonb_build_object('timestamp',$5::text,'signaturePrefix',$6::text),$7,now())
		ON CONFLICT(endpoint_id,event_id,direction) DO NOTHING RETURNING id`, tenant, endpointID, "transaction."+transaction.Status, eventID, timestamp, truncate(signature, 12), json.RawMessage(body)).Scan(&deliveryID)
	if errors.Is(err, pgx.ErrNoRows) {
		write(w, 200, envelope{Success: true, Data: map[string]any{"duplicate": true}})
		return
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось зарегистрировать webhook")
		return
	}
	result, ingestErr := a.integrationService.Ingest(r.Context(), transaction)
	if ingestErr != nil {
		_, _ = a.db.Exec(r.Context(), `UPDATE webhook_deliveries SET status='failed',attempts=attempts+1,last_error=$2,next_attempt_at=now()+interval '1 minute' WHERE id=$1 AND company_id=$3`, deliveryID, ingestErr.Error(), tenant)
		fail(w, 422, "INGEST_FAILED", "Не удалось обработать POS-транзакцию")
		return
	}
	_, _ = a.db.Exec(r.Context(), `UPDATE webhook_deliveries SET status='succeeded',attempts=attempts+1,processed_at=now() WHERE id=$1 AND company_id=$2`, deliveryID, tenant)
	_, _ = a.db.Exec(r.Context(), `UPDATE webhook_endpoints SET last_delivery_at=now(),last_success_at=now(),failure_count=0,updated_at=now() WHERE id=$1 AND company_id=$2`, endpointID, tenant)
	write(w, 202, envelope{Success: true, Data: result})
}

func (a *api) listWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	connectionID := strings.TrimSpace(r.URL.Query().Get("connectionId"))
	rows, err := a.db.Query(r.Context(), `SELECT d.id,d.event_type,d.event_id,d.direction,d.status,d.response_status,d.attempts,d.next_attempt_at,d.processed_at,coalesce(d.last_error,''),d.created_at,e.name
		FROM webhook_deliveries d JOIN webhook_endpoints e ON e.id=d.endpoint_id WHERE d.company_id=$1
		AND ($2='' OR e.connection_id=nullif($2,'')::uuid) ORDER BY d.created_at DESC LIMIT 100`, companyID(r), connectionID)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить журнал webhook")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, eventType, eventID, direction, status, lastError, endpoint string
		var responseStatus *int
		var attempts int
		var nextAttempt, createdAt time.Time
		var processedAt *time.Time
		if err = rows.Scan(&id, &eventType, &eventID, &direction, &status, &responseStatus, &attempts, &nextAttempt, &processedAt, &lastError, &createdAt, &endpoint); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось прочитать журнал webhook")
			return
		}
		items = append(items, map[string]any{"id": id, "endpoint": endpoint, "eventType": eventType, "eventId": eventID, "direction": direction, "status": status, "responseStatus": responseStatus, "attempts": attempts, "nextAttemptAt": nextAttempt, "processedAt": processedAt, "lastError": lastError, "createdAt": createdAt})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось прочитать журнал webhook")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
