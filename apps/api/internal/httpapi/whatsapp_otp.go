package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type otpRequestInput struct {
	Company string `json:"company"`
	Phone   string `json:"phone"`
}
type otpVerifyInput struct {
	Company string `json:"company"`
	Phone   string `json:"phone"`
	Code    string `json:"code"`
}
type otpRecord struct {
	CustomerID string `json:"customerId"`
	CompanyID  string `json:"companyId"`
	CodeHash   string `json:"codeHash"`
	Attempts   int    `json:"attempts"`
}

var nonDigits = regexp.MustCompile(`\D`)

func otpKey(company, phone string) string {
	return "customer-otp:" + strings.ToLower(strings.TrimSpace(company)) + ":" + nonDigits.ReplaceAllString(phone, "")
}
func otpHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func (a *api) customerOTPRequest(w http.ResponseWriter, r *http.Request) {
	var in otpRequestInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Company) == "" || len(nonDigits.ReplaceAllString(in.Phone, "")) < 10 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите компанию и корректный телефон")
		return
	}
	var customerID, companyID string
	err := a.db.QueryRow(r.Context(), `SELECT c.id,c.company_id FROM customers c JOIN companies co ON co.id=c.company_id AND co.status='active' WHERE co.slug=$1 AND regexp_replace(c.phone,'\D','','g')=$2 AND c.deleted_at IS NULL`, strings.ToLower(strings.TrimSpace(in.Company)), nonDigits.ReplaceAllString(in.Phone, "")).Scan(&customerID, &companyID)
	response := map[string]any{"sent": true, "expiresIn": 300, "channel": "whatsapp"}
	if err == nil {
		buf := make([]byte, 3)
		if _, e := rand.Read(buf); e != nil {
			fail(w, 500, "OTP_ERROR", "Не удалось создать код")
			return
		}
		code := fmt.Sprintf("%06d", int(buf[0])<<16|int(buf[1])<<8|int(buf[2]))
		code = code[len(code)-6:]
		record, _ := json.Marshal(otpRecord{CustomerID: customerID, CompanyID: companyID, CodeHash: otpHash(code)})
		if e := a.redis.Set(r.Context(), otpKey(in.Company, in.Phone), record, 5*time.Minute).Err(); e != nil {
			fail(w, 500, "OTP_ERROR", "Не удалось сохранить код")
			return
		}
		if e := a.sendWhatsAppOTP(r, nonDigits.ReplaceAllString(in.Phone, ""), code); e != nil {
			if !a.otpDevMode {
				a.redis.Del(r.Context(), otpKey(in.Company, in.Phone))
				fail(w, 503, "WHATSAPP_UNAVAILABLE", "WhatsApp временно недоступен. Используйте резервный вход по PIN")
				return
			}
			response["delivery"] = "development"
			response["devCode"] = code
		}
	}
	write(w, 200, envelope{Success: true, Data: response})
}

func (a *api) customerOTPVerify(w http.ResponseWriter, r *http.Request) {
	var in otpVerifyInput
	if !decode(w, r, &in) {
		return
	}
	key := otpKey(in.Company, in.Phone)
	raw, err := a.redis.Get(r.Context(), key).Bytes()
	if err != nil {
		fail(w, 401, "OTP_EXPIRED", "Код истёк. Запросите новый")
		return
	}
	var record otpRecord
	if json.Unmarshal(raw, &record) != nil {
		fail(w, 401, "OTP_INVALID", "Неверный код")
		return
	}
	record.Attempts++
	if record.Attempts > 5 {
		a.redis.Del(r.Context(), key)
		fail(w, 429, "OTP_ATTEMPTS_EXCEEDED", "Слишком много попыток. Запросите новый код")
		return
	}
	if record.CodeHash != otpHash(strings.TrimSpace(in.Code)) {
		updated, _ := json.Marshal(record)
		ttl := a.redis.TTL(r.Context(), key).Val()
		a.redis.Set(r.Context(), key, updated, ttl)
		fail(w, 401, "OTP_INVALID", "Неверный код")
		return
	}
	a.redis.Del(r.Context(), key)
	access, refresh, err := a.issueTokens(r, record.CustomerID, record.CompanyID, "customer")
	if err != nil {
		fail(w, 500, "TOKEN_ERROR", "Не удалось создать сессию")
		return
	}
	a.setSessionCookies(w, access, refresh, "customer")
	write(w, 200, envelope{Success: true, Data: map[string]bool{"authenticated": true}})
}

func (a *api) sendWhatsAppOTP(r *http.Request, phone, code string) error {
	if a.whatsappToken == "" || a.whatsappPhoneID == "" {
		return fmt.Errorf("whatsapp is not configured")
	}
	payload := map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": phone, "type": "template", "template": map[string]any{"name": a.whatsappTemplate, "language": map[string]string{"code": "ru"}, "components": []any{map[string]any{"type": "body", "parameters": []any{map[string]string{"type": "text", "text": code}}}, map[string]any{"type": "button", "sub_type": "url", "index": "0", "parameters": []any{map[string]string{"type": "text", "text": code}}}}}}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/%s/%s/messages", strings.TrimRight(a.whatsappAPIBase, "/"), a.whatsappGraphVersion, a.whatsappPhoneID)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.whatsappToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("meta status %d: %s", resp.StatusCode, string(detail))
	}
	return nil
}

func sendWhatsAppText(ctx context.Context, phone, message string) (string, error) {
	token, phoneID := os.Getenv("WHATSAPP_ACCESS_TOKEN"), os.Getenv("WHATSAPP_PHONE_NUMBER_ID")
	if token == "" || phoneID == "" {
		return "", fmt.Errorf("whatsapp is not configured")
	}
	phone = nonDigits.ReplaceAllString(phone, "")
	if len(phone) < 10 {
		return "", fmt.Errorf("customer has no valid WhatsApp phone")
	}
	payload, _ := json.Marshal(map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": phone, "type": "text", "text": map[string]any{"preview_url": false, "body": message}})
	base := strings.TrimRight(envValue("WHATSAPP_API_BASE", "https://graph.facebook.com"), "/")
	url := fmt.Sprintf("%s/%s/%s/messages", base, envValue("WHATSAPP_GRAPH_VERSION", "v23.0"), phoneID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("meta status %d: %s", response.StatusCode, strings.TrimSpace(string(detail)))
	}
	var result struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if json.Unmarshal(detail, &result) != nil || len(result.Messages) == 0 || result.Messages[0].ID == "" {
		return "", fmt.Errorf("meta response has no message id")
	}
	return result.Messages[0].ID, nil
}

func validMetaSignature(secret string, body []byte, signature string) bool {
	if secret == "" || !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write(body)
	return hmac.Equal(provided, digest.Sum(nil))
}

func (a *api) whatsAppStatusWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("hub.mode") == "subscribe" && r.URL.Query().Get("hub.verify_token") == os.Getenv("WHATSAPP_VERIFY_TOKEN") && os.Getenv("WHATSAPP_VERIFY_TOKEN") != "" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		fail(w, 403, "WEBHOOK_VERIFICATION_FAILED", "Webhook verification failed")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fail(w, 400, "INVALID_WEBHOOK", "Invalid payload")
		return
	}
	if !validMetaSignature(os.Getenv("WHATSAPP_APP_SECRET"), body, r.Header.Get("X-Hub-Signature-256")) {
		fail(w, 401, "INVALID_SIGNATURE", "Invalid webhook signature")
		return
	}
	var payload struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Statuses []struct {
						ID        string          `json:"id"`
						Status    string          `json:"status"`
						Timestamp string          `json:"timestamp"`
						Errors    json.RawMessage `json:"errors"`
					} `json:"statuses"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &payload) != nil {
		fail(w, 400, "INVALID_WEBHOOK", "Invalid payload")
		return
	}
	updated := 0
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, status := range change.Value.Statuses {
				if status.ID == "" {
					continue
				}
				allowed := status.Status == "sent" || status.Status == "delivered" || status.Status == "read" || status.Status == "failed"
				if !allowed {
					continue
				}
				tag, _ := a.db.Exec(r.Context(), `UPDATE campaign_automation_runs SET provider_status=$2,delivered_at=CASE WHEN $2 IN('delivered','read') THEN coalesce(delivered_at,now()) ELSE delivered_at END,error=CASE WHEN $2='failed' THEN coalesce(nullif($3::text,'null'),'WhatsApp delivery failed') ELSE error END,provider_payload=$4::jsonb,updated_at=now() WHERE provider_message_id=$1`, status.ID, status.Status, string(status.Errors), body)
				updated += int(tag.RowsAffected())
			}
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]int{"updated": updated}})
}
