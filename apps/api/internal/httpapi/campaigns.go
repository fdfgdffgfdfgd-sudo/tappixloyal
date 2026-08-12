package httpapi

import (
	"crypto/sha256"
	"encoding/binary"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type campaignInput struct {
	Name                  string  `json:"name"`
	Subject               string  `json:"subject"`
	Body                  string  `json:"body"`
	Segment               string  `json:"segment"`
	InactiveDays          int     `json:"inactiveDays"`
	Level                 string  `json:"level"`
	MessageCost           float64 `json:"messageCost"`
	RewardCost            float64 `json:"rewardCost"`
	AttributionWindowDays int     `json:"attributionWindowDays"`
	HoldoutPercent        int     `json:"holdoutPercent"`
}

func (a *api) listCampaigns(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id,name,channel,subject,segment,status,audience_count,sent_count,failed_count,message_cost,reward_cost,attribution_window_days,holdout_percent,created_at,sent_at FROM marketing_campaigns WHERE company_id=$1 ORDER BY created_at DESC LIMIT 100`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить кампании")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, channel, subject, segment, status string
		var audience, sent, failed, attributionDays, holdoutPercent int
		var messageCost, rewardCost float64
		var created time.Time
		var sentAt *time.Time
		if rows.Scan(&id, &name, &channel, &subject, &segment, &status, &audience, &sent, &failed, &messageCost, &rewardCost, &attributionDays, &holdoutPercent, &created, &sentAt) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "channel": channel, "subject": subject, "segment": segment, "status": status, "audienceCount": audience, "sentCount": sent, "failedCount": failed, "messageCost": messageCost, "rewardCost": rewardCost, "attributionWindowDays": attributionDays, "holdoutPercent": holdoutPercent, "createdAt": created, "sentAt": sentAt})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
func (a *api) createCampaign(w http.ResponseWriter, r *http.Request) {
	var in campaignInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Subject) == "" || strings.ContainsAny(in.Subject, "\r\n") || strings.TrimSpace(in.Body) == "" || !validSegment(in.Segment) {
		fail(w, 422, "VALIDATION_ERROR", "Заполните название, тему, сообщение и сегмент")
		return
	}
	if in.InactiveDays <= 0 {
		in.InactiveDays = 30
	}
	if in.AttributionWindowDays == 0 {
		in.AttributionWindowDays = 7
	}
	if in.AttributionWindowDays < 1 || in.AttributionWindowDays > 90 || in.MessageCost < 0 || in.RewardCost < 0 || (in.HoldoutPercent != 0 && (in.HoldoutPercent < 5 || in.HoldoutPercent > 10)) {
		fail(w, 422, "INVALID_CAMPAIGN_ECONOMICS", "Окно атрибуции 1–90 дней, holdout — 0 или 5–10%")
		return
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO marketing_campaigns(company_id,name,subject,body,segment,segment_settings,message_cost,reward_cost,attribution_window_days,holdout_percent,created_by) VALUES($1,$2,$3,$4,$5,jsonb_build_object('inactiveDays',$6::integer,'level',$7::text),$8,$9,$10,$11,$12) RETURNING id`, companyID(r), strings.TrimSpace(in.Name), strings.TrimSpace(in.Subject), strings.TrimSpace(in.Body), in.Segment, in.InactiveDays, in.Level, in.MessageCost, in.RewardCost, in.AttributionWindowDays, in.HoldoutPercent, claims.Subject).Scan(&id)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось создать кампанию")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]string{"id": id}})
}
func validSegment(s string) bool {
	return s == "all" || s == "inactive" || s == "birthday" || s == "level"
}
func (a *api) campaignAudience(r *http.Request, id string) ([]map[string]string, error) {
	var segment string
	var settings map[string]any
	err := a.db.QueryRow(r.Context(), `SELECT segment,segment_settings FROM marketing_campaigns WHERE company_id=$1 AND id=$2`, companyID(r), id).Scan(&segment, &settings)
	if err != nil {
		return nil, err
	}
	days := 30
	if v, ok := settings["inactiveDays"].(float64); ok {
		days = int(v)
	}
	level, _ := settings["level"].(string)
	rows, err := a.db.Query(r.Context(), `SELECT c.id,c.first_name,c.last_name,c.email FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND c.email IS NOT NULL AND c.email<>'' AND ($2='all' OR ($2='birthday' AND extract(month from c.birthday)=extract(month from current_date)) OR ($2='level' AND c.level=$4) OR ($2='inactive' AND coalesce((SELECT max(v.created_at) FROM visits v WHERE v.company_id=c.company_id AND v.customer_id=c.id),c.created_at)<now()-make_interval(days=>$3))) ORDER BY c.created_at DESC LIMIT 500`, companyID(r), segment, days, level)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var cid, first, last, email string
		if rows.Scan(&cid, &first, &last, &email) == nil {
			items = append(items, map[string]string{"id": cid, "name": strings.TrimSpace(first + " " + last), "email": email})
		}
	}
	return items, nil
}
func (a *api) previewCampaign(w http.ResponseWriter, r *http.Request) {
	items, err := a.campaignAudience(r, r.PathValue("id"))
	if err != nil {
		if err == pgx.ErrNoRows {
			fail(w, 404, "CAMPAIGN_NOT_FOUND", "Кампания не найдена")
		} else {
			fail(w, 500, "DATABASE_ERROR", "Не удалось собрать аудиторию")
		}
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"total": len(items), "items": items}})
}
func (a *api) sendCampaign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var subject, body, status, holdoutSeed string
	var holdoutPercent int
	if a.db.QueryRow(r.Context(), `SELECT subject,body,status,holdout_percent,holdout_seed FROM marketing_campaigns WHERE company_id=$1 AND id=$2`, companyID(r), id).Scan(&subject, &body, &status, &holdoutPercent, &holdoutSeed) != nil {
		fail(w, 404, "CAMPAIGN_NOT_FOUND", "Кампания не найдена")
		return
	}
	if status != "draft" {
		fail(w, 409, "CAMPAIGN_ALREADY_SENT", "Кампания уже отправлялась")
		return
	}
	audience, err := a.campaignAudience(r, id)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось собрать аудиторию")
		return
	}
	if len(audience) == 0 {
		fail(w, 409, "AUDIENCE_EMPTY", "В выбранном сегменте нет клиентов с email")
		return
	}
	_, _ = a.db.Exec(r.Context(), `UPDATE marketing_campaigns SET status='sending',audience_count=$3 WHERE company_id=$1 AND id=$2`, companyID(r), id, len(audience))
	sent, failed := 0, 0
	holdout := 0
	for _, x := range audience {
		if campaignHoldout(holdoutSeed, x["id"], holdoutPercent) {
			holdout++
			_, _ = a.db.Exec(r.Context(), `INSERT INTO campaign_recipients(company_id,campaign_id,customer_id,recipient,status,experiment_group) VALUES($1,$2,$3,$4,'held_out','holdout') ON CONFLICT(campaign_id,customer_id) DO NOTHING`, companyID(r), id, x["id"], x["email"])
			continue
		}
		personal := strings.ReplaceAll(body, "{{name}}", x["name"])
		e := sendEmailWithTimeout(r.Context(), x["email"], subject, personal)
		recipientStatus, errorText := "sent", ""
		var sentAt any = time.Now()
		if e != nil {
			recipientStatus = "failed"
			errorText = e.Error()
			sentAt = nil
			failed++
		} else {
			sent++
		}
		var recipientID string
		_ = a.db.QueryRow(r.Context(), `INSERT INTO campaign_recipients(company_id,campaign_id,customer_id,recipient,status,error,sent_at,delivered_at,experiment_group) VALUES($1,$2,$3,$4,$5,nullif($6,''),$7,$7,'treatment') ON CONFLICT(campaign_id,customer_id) DO UPDATE SET status=excluded.status,error=excluded.error,sent_at=excluded.sent_at,delivered_at=excluded.delivered_at RETURNING id`, companyID(r), id, x["id"], x["email"], recipientStatus, errorText, sentAt).Scan(&recipientID)
		if recipientStatus == "sent" {
			_, _ = a.db.Exec(r.Context(), `INSERT INTO campaign_conversions(company_id,campaign_id,campaign_recipient_id,customer_id,conversion_type,idempotency_key) VALUES($1,$2,$3,$4,'delivered',$5) ON CONFLICT DO NOTHING`, companyID(r), id, recipientID, x["id"], "campaign-delivered:"+recipientID)
		}
	}
	final := "sent"
	if failed > 0 && sent > 0 {
		final = "partial"
	} else if failed > 0 {
		final = "failed"
	}
	_, _ = a.db.Exec(r.Context(), `UPDATE marketing_campaigns SET status=$3,sent_count=$4,failed_count=$5,sent_at=now() WHERE company_id=$1 AND id=$2`, companyID(r), id, final, sent, failed)
	write(w, 200, envelope{Success: true, Data: map[string]any{"status": final, "audience": len(audience), "sent": sent, "failed": failed, "holdout": holdout}})
}

func campaignHoldout(seed, customerID string, percent int) bool {
	if percent <= 0 {
		return false
	}
	digest := sha256.Sum256([]byte(seed + ":" + customerID))
	return int(binary.BigEndian.Uint32(digest[:4])%100) < percent
}
