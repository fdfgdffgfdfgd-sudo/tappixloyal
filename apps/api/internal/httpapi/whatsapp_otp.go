package httpapi

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	write(w, 200, envelope{Success: true, Data: map[string]string{"accessToken": access, "refreshToken": refresh}})
}

func (a *api) sendWhatsAppOTP(r *http.Request, phone, code string) error {
	if a.whatsappToken == "" || a.whatsappPhoneID == "" {
		return fmt.Errorf("whatsapp is not configured")
	}
	payload := map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": phone, "type": "template", "template": map[string]any{"name": a.whatsappTemplate, "language": map[string]string{"code": "ru"}, "components": []any{map[string]any{"type": "body", "parameters": []any{map[string]string{"type": "text", "text": code}}}, map[string]any{"type": "button", "sub_type": "url", "index": "0", "parameters": []any{map[string]string{"type": "text", "text": code}}}}}}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", a.whatsappGraphVersion, a.whatsappPhoneID)
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
