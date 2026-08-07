package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var allowedContentTypes = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "application/pdf": ".pdf"}

type integrationInput struct {
	TelegramEnabled bool   `json:"telegramEnabled"`
	SMSEnabled      bool   `json:"smsEnabled"`
	WebhookURL      string `json:"webhookUrl"`
	CRMName         string `json:"crmName"`
}

func (a *api) uploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		fail(w, 413, "FILE_TOO_LARGE", "Файл превышает 10 МБ")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, 422, "FILE_REQUIRED", "Выберите файл")
		return
	}
	defer file.Close()
	head := make([]byte, 512)
	n, _ := file.Read(head)
	contentType := http.DetectContentType(head[:n])
	extension, ok := allowedContentTypes[contentType]
	if !ok {
		fail(w, 422, "FILE_TYPE_NOT_ALLOWED", "Разрешены JPG, PNG, WebP и PDF")
		return
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		fail(w, 500, "FILE_ERROR", "Не удалось прочитать файл")
		return
	}
	random := make([]byte, 20)
	_, _ = rand.Read(random)
	storageName := hex.EncodeToString(random) + extension
	directory := envValue("UPLOAD_DIR", "/data/uploads")
	if err = os.MkdirAll(directory, 0750); err != nil {
		fail(w, 500, "STORAGE_ERROR", "Хранилище недоступно")
		return
	}
	destination := filepath.Join(directory, storageName)
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		fail(w, 500, "STORAGE_ERROR", "Не удалось сохранить файл")
		return
	}
	size, copyErr := io.Copy(output, file)
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil || size <= 0 || size > 10<<20 {
		_ = os.Remove(destination)
		fail(w, 500, "FILE_ERROR", "Не удалось сохранить файл")
		return
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	kind := r.FormValue("kind")
	if kind == "" {
		kind = "asset"
	}
	var id string
	err = a.db.QueryRow(r.Context(), `INSERT INTO files(company_id,uploaded_by,kind,original_name,storage_name,content_type,size_bytes) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, claims.CompanyID, claims.Subject, kind, filepath.Base(header.Filename), storageName, contentType, size).Scan(&id)
	if err != nil {
		_ = os.Remove(destination)
		fail(w, 500, "DATABASE_ERROR", "Не удалось зарегистрировать файл")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "name": header.Filename, "contentType": contentType, "size": size, "url": "/api/v1/public/files/" + id}})
}
func (a *api) publicFile(w http.ResponseWriter, r *http.Request) {
	var storageName, contentType string
	err := a.db.QueryRow(r.Context(), `SELECT storage_name,content_type FROM files WHERE id=$1 AND deleted_at IS NULL`, r.PathValue("id")).Scan(&storageName, &contentType)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(envValue("UPLOAD_DIR", "/data/uploads"), filepath.Base(storageName))
	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func (a *api) listFiles(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id,kind,original_name,content_type,size_bytes,created_at FROM files WHERE company_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить файлы")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, kind, name, contentType string
		var size int64
		var created time.Time
		if rows.Scan(&id, &kind, &name, &contentType, &size, &created) == nil {
			items = append(items, map[string]any{"id": id, "kind": kind, "name": name, "contentType": contentType, "size": size, "createdAt": created, "url": "/api/v1/public/files/" + id})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) deleteFile(w http.ResponseWriter, r *http.Request) {
	var storage string
	err := a.db.QueryRow(r.Context(), `UPDATE files SET deleted_at=now() WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL RETURNING storage_name`, companyID(r), r.PathValue("id")).Scan(&storage)
	if err != nil {
		fail(w, 404, "FILE_NOT_FOUND", "Файл не найден")
		return
	}
	_ = os.Remove(filepath.Join(envValue("UPLOAD_DIR", "/data/uploads"), filepath.Base(storage)))
	write(w, 200, envelope{Success: true, Data: map[string]bool{"deleted": true}})
}
func (a *api) getIntegrations(w http.ResponseWriter, r *http.Request) {
	var in integrationInput
	err := a.db.QueryRow(r.Context(), `SELECT telegram_enabled,sms_enabled,coalesce(webhook_url,''),coalesce(crm_name,'') FROM integration_settings WHERE company_id=$1`, companyID(r)).Scan(&in.TelegramEnabled, &in.SMSEnabled, &in.WebhookURL, &in.CRMName)
	if err != nil {
		write(w, 200, envelope{Success: true, Data: in})
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}
func (a *api) updateIntegrations(w http.ResponseWriter, r *http.Request) {
	var in integrationInput
	if !decode(w, r, &in) {
		return
	}
	if in.WebhookURL != "" && !strings.HasPrefix(in.WebhookURL, "https://") {
		fail(w, 422, "VALIDATION_ERROR", "Webhook должен использовать HTTPS")
		return
	}
	_, err := a.db.Exec(r.Context(), `INSERT INTO integration_settings(company_id,telegram_enabled,sms_enabled,webhook_url,crm_name) VALUES($1,$2,$3,$4,$5) ON CONFLICT(company_id) DO UPDATE SET telegram_enabled=excluded.telegram_enabled,sms_enabled=excluded.sms_enabled,webhook_url=excluded.webhook_url,crm_name=excluded.crm_name,updated_at=now()`, companyID(r), in.TelegramEnabled, in.SMSEnabled, in.WebhookURL, in.CRMName)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось сохранить интеграции")
		return
	}
	write(w, 200, envelope{Success: true, Data: in})
}
