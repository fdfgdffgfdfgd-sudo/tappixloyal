package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type automationUpdateInput struct {
	Channel  string         `json:"channel"`
	Subject  string         `json:"subject"`
	Message  string         `json:"message"`
	Settings map[string]any `json:"settings"`
	Active   *bool          `json:"active"`
}

func (a *api) listCampaignAutomations(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT a.id,a.trigger_type,a.name,a.channel,a.subject,a.message,a.settings,a.is_active,a.updated_at,
		count(run.id),count(run.id) FILTER(WHERE run.status='sent'),count(run.id) FILTER(WHERE run.status='failed')
		FROM campaign_automations a LEFT JOIN campaign_automation_runs run ON run.automation_id=a.id AND run.company_id=a.company_id
		WHERE a.company_id=$1 GROUP BY a.id ORDER BY a.trigger_type`, companyID(r))
	if err != nil {
		fail(w, 500, "AUTOMATIONS_FAILED", "Не удалось загрузить автоматизации")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, trigger, name, channel, subject, message string
		var settings json.RawMessage
		var active bool
		var updated time.Time
		var runs, sent, failed int
		if err := rows.Scan(&id, &trigger, &name, &channel, &subject, &message, &settings, &active, &updated, &runs, &sent, &failed); err != nil {
			fail(w, 500, "AUTOMATIONS_FAILED", "Не удалось загрузить автоматизации")
			return
		}
		items = append(items, map[string]any{"id": id, "triggerType": trigger, "name": name, "channel": channel, "subject": subject, "message": message, "settings": settings, "active": active, "updatedAt": updated, "runs": runs, "sent": sent, "failed": failed})
	}
	if rows.Err() != nil {
		fail(w, 500, "AUTOMATIONS_FAILED", "Не удалось загрузить автоматизации")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) updateCampaignAutomation(w http.ResponseWriter, r *http.Request) {
	var in automationUpdateInput
	if !decode(w, r, &in) {
		return
	}
	in.Channel = strings.ToLower(strings.TrimSpace(in.Channel))
	if in.Channel != "email" && in.Channel != "whatsapp" {
		fail(w, 422, "INVALID_CHANNEL", "Канал должен быть email или whatsapp")
		return
	}
	if strings.TrimSpace(in.Message) == "" {
		fail(w, 422, "MESSAGE_REQUIRED", "Введите текст сообщения")
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE campaign_automations SET channel=$3,subject=$4,message=$5,settings=$6,is_active=$7,updated_at=now() WHERE company_id=$1 AND id=$2`, companyID(r), r.PathValue("id"), in.Channel, strings.TrimSpace(in.Subject), strings.TrimSpace(in.Message), in.Settings, active)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "AUTOMATION_NOT_FOUND", "Автоматизация не найдена")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"updated": true, "active": active}})
}

func (a *api) runCampaignAutomations(w http.ResponseWriter, r *http.Request) {
	bonuses, err := processBirthdayBonuses(r.Context(), a.db, companyID(r))
	if err != nil {
		fail(w, 500, "AUTOMATION_FAILED", "Не удалось начислить birthday-бонусы")
		return
	}
	messages, err := processCampaignAutomations(r.Context(), a.db, companyID(r))
	if err != nil {
		slog.Error("campaign automation processing failed", "company", companyID(r), "error", err)
		fail(w, 500, "AUTOMATION_FAILED", "Не удалось выполнить триггерные рассылки")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]int{"birthdayBonuses": bonuses, "messagesSent": messages}})
}
