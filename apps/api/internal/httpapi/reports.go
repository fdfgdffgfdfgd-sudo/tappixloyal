package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type reportScheduleInput struct {
	Name         string   `json:"name"`
	ReportType   string   `json:"reportType"`
	Channel      string   `json:"channel"`
	Recipients   []string `json:"recipients"`
	Frequency    string   `json:"frequency"`
	Timezone     string   `json:"timezone"`
	SendHour     int      `json:"sendHour"`
	SendWeekday  *int     `json:"sendWeekday"`
	SendMonthday *int     `json:"sendMonthday"`
	Format       string   `json:"format"`
	Active       bool     `json:"active"`
}

type reportData struct {
	Company         string
	Period          string
	Customers       int
	ActiveCustomers int
	RepeatCustomers int
	Transactions    int
	Revenue         float64
	MemberRevenue   float64
	BonusLiability  float64
	AtRisk          int
	CampaignRevenue float64
}

func (a *api) listReportSchedules(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id,name,report_type,channel,recipients,frequency,timezone,send_hour,send_weekday,send_monthday,format,is_active,next_run_at,last_run_at,last_status,coalesce(last_error,''),created_at FROM report_schedules WHERE company_id=$1 ORDER BY created_at DESC`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить расписания")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, kind, channel, frequency, timezone, format, status, lastError string
		var recipients []byte
		var hour int
		var weekday, monthday *int
		var active bool
		var next, last *time.Time
		var created time.Time
		if rows.Scan(&id, &name, &kind, &channel, &recipients, &frequency, &timezone, &hour, &weekday, &monthday, &format, &active, &next, &last, &status, &lastError, &created) == nil {
			var targets []string
			_ = json.Unmarshal(recipients, &targets)
			items = append(items, map[string]any{"id": id, "name": name, "reportType": kind, "channel": channel, "recipients": targets, "frequency": frequency, "timezone": timezone, "sendHour": hour, "sendWeekday": weekday, "sendMonthday": monthday, "format": format, "active": active, "nextRunAt": next, "lastRunAt": last, "lastStatus": status, "lastError": lastError, "createdAt": created})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func validateReportInput(in *reportScheduleInput) string {
	in.Name = strings.TrimSpace(in.Name)
	in.ReportType = strings.TrimSpace(in.ReportType)
	in.Channel = strings.ToLower(strings.TrimSpace(in.Channel))
	in.Frequency = strings.ToLower(strings.TrimSpace(in.Frequency))
	in.Format = strings.ToLower(strings.TrimSpace(in.Format))
	in.Timezone = strings.TrimSpace(in.Timezone)
	if in.Timezone == "" {
		in.Timezone = "Asia/Almaty"
	}
	if in.ReportType == "" {
		in.ReportType = "owner_summary"
	}
	if in.Name == "" || len(in.Name) > 160 {
		return "Укажите название отчёта"
	}
	if in.Channel != "email" && in.Channel != "whatsapp" && in.Channel != "webhook" {
		return "Выберите email, WhatsApp или webhook"
	}
	if in.Frequency != "daily" && in.Frequency != "weekly" && in.Frequency != "monthly" {
		return "Выберите частоту отчёта"
	}
	if in.Format != "summary" && in.Format != "csv" && in.Format != "xlsx" && in.Format != "pdf" {
		return "Выберите формат отчёта"
	}
	if in.SendHour < 0 || in.SendHour > 23 || len(in.Recipients) == 0 {
		return "Укажите время и хотя бы одного получателя"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return "Неизвестный часовой пояс"
	}
	if in.Frequency == "weekly" && (in.SendWeekday == nil || *in.SendWeekday < 1 || *in.SendWeekday > 7) {
		return "Укажите день недели"
	}
	if in.Frequency == "monthly" && (in.SendMonthday == nil || *in.SendMonthday < 1 || *in.SendMonthday > 28) {
		return "Укажите день месяца от 1 до 28"
	}
	for i := range in.Recipients {
		in.Recipients[i] = strings.TrimSpace(in.Recipients[i])
		if in.Recipients[i] == "" {
			return "Получатель не может быть пустым"
		}
	}
	return ""
}

func nextReportRun(in reportScheduleInput, now time.Time) time.Time {
	loc, _ := time.LoadLocation(in.Timezone)
	local := now.In(loc)
	candidate := time.Date(local.Year(), local.Month(), local.Day(), in.SendHour, 0, 0, 0, loc)
	switch in.Frequency {
	case "daily":
		if !candidate.After(local) {
			candidate = candidate.AddDate(0, 0, 1)
		}
	case "weekly":
		for (int(candidate.Weekday())+6)%7+1 != *in.SendWeekday || !candidate.After(local) {
			candidate = candidate.AddDate(0, 0, 1)
		}
	case "monthly":
		candidate = time.Date(local.Year(), local.Month(), *in.SendMonthday, in.SendHour, 0, 0, 0, loc)
		if !candidate.After(local) {
			candidate = candidate.AddDate(0, 1, 0)
		}
	}
	return candidate.UTC()
}

func (a *api) createReportSchedule(w http.ResponseWriter, r *http.Request) {
	var in reportScheduleInput
	if !decode(w, r, &in) {
		return
	}
	if message := validateReportInput(&in); message != "" {
		fail(w, 422, "VALIDATION_ERROR", message)
		return
	}
	recipients, _ := json.Marshal(in.Recipients)
	next := nextReportRun(in, time.Now())
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO report_schedules(company_id,name,report_type,channel,recipients,frequency,timezone,send_hour,send_weekday,send_monthday,format,is_active,next_run_at,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`, companyID(r), in.Name, in.ReportType, in.Channel, recipients, in.Frequency, in.Timezone, in.SendHour, in.SendWeekday, in.SendMonthday, in.Format, in.Active, next, identity(r).Subject).Scan(&id)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось создать расписание")
		return
	}
	a.auditReport(r, "report.schedule.created", id, map[string]any{"channel": in.Channel, "frequency": in.Frequency, "format": in.Format})
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "nextRunAt": next}})
}

func (a *api) updateReportSchedule(w http.ResponseWriter, r *http.Request) {
	var in reportScheduleInput
	if !decode(w, r, &in) {
		return
	}
	if message := validateReportInput(&in); message != "" {
		fail(w, 422, "VALIDATION_ERROR", message)
		return
	}
	recipients, _ := json.Marshal(in.Recipients)
	next := nextReportRun(in, time.Now())
	tag, err := a.db.Exec(r.Context(), `UPDATE report_schedules SET name=$3,report_type=$4,channel=$5,recipients=$6,frequency=$7,timezone=$8,send_hour=$9,send_weekday=$10,send_monthday=$11,format=$12,is_active=$13,next_run_at=$14,updated_at=now() WHERE id=$2 AND company_id=$1`, companyID(r), r.PathValue("id"), in.Name, in.ReportType, in.Channel, recipients, in.Frequency, in.Timezone, in.SendHour, in.SendWeekday, in.SendMonthday, in.Format, in.Active, next)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "REPORT_NOT_FOUND", "Расписание не найдено")
		return
	}
	a.auditReport(r, "report.schedule.updated", r.PathValue("id"), map[string]any{"active": in.Active})
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": r.PathValue("id"), "nextRunAt": next}})
}

func (a *api) deleteReportSchedule(w http.ResponseWriter, r *http.Request) {
	tag, err := a.db.Exec(r.Context(), `DELETE FROM report_schedules WHERE id=$2 AND company_id=$1`, companyID(r), r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "REPORT_NOT_FOUND", "Расписание не найдено")
		return
	}
	a.auditReport(r, "report.schedule.deleted", r.PathValue("id"), nil)
	write(w, 200, envelope{Success: true, Data: map[string]bool{"deleted": true}})
}

func (a *api) runReportSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := queueReportRun(r.Context(), a.db, companyID(r), r.PathValue("id"), "manual:"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err != nil {
		fail(w, 404, "REPORT_NOT_FOUND", "Расписание не найдено")
		return
	}
	a.auditReport(r, "report.run.queued", id, nil)
	write(w, 202, envelope{Success: true, Data: map[string]string{"id": id, "status": "queued"}})
}

func (a *api) retryReportRun(w http.ResponseWriter, r *http.Request) {
	var schedule string
	err := a.db.QueryRow(r.Context(), `SELECT schedule_id FROM report_runs WHERE id=$2 AND company_id=$1`, companyID(r), r.PathValue("id")).Scan(&schedule)
	if err != nil {
		fail(w, 404, "REPORT_RUN_NOT_FOUND", "Запуск не найден")
		return
	}
	id, err := queueReportRun(r.Context(), a.db, companyID(r), schedule, "retry:"+r.PathValue("id")+":"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось поставить повтор в очередь")
		return
	}
	a.auditReport(r, "report.run.retried", id, map[string]any{"previousRunId": r.PathValue("id")})
	write(w, 202, envelope{Success: true, Data: map[string]string{"id": id, "status": "queued"}})
}

func (a *api) listReportRuns(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT rr.id,rr.schedule_id,rs.name,rr.status,rr.format,coalesce(rr.filename,''),rr.attempts,coalesce(rr.error,''),rr.created_at,rr.completed_at,rr.next_attempt_at,(rr.artifact IS NOT NULL) FROM report_runs rr JOIN report_schedules rs ON rs.id=rr.schedule_id WHERE rr.company_id=$1 ORDER BY rr.created_at DESC LIMIT 100`, companyID(r))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить историю отчётов")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, schedule, name, status, format, filename, errorText string
		var attempts int
		var created, nextAttempt time.Time
		var completed *time.Time
		var downloadable bool
		if rows.Scan(&id, &schedule, &name, &status, &format, &filename, &attempts, &errorText, &created, &completed, &nextAttempt, &downloadable) == nil {
			items = append(items, map[string]any{"id": id, "scheduleId": schedule, "name": name, "status": status, "format": format, "filename": filename, "attempts": attempts, "error": errorText, "createdAt": created, "completedAt": completed, "nextAttemptAt": nextAttempt, "downloadable": downloadable})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) downloadReportRun(w http.ResponseWriter, r *http.Request) {
	var filename, mime string
	var artifact []byte
	err := a.db.QueryRow(r.Context(), `SELECT coalesce(filename,''),coalesce(mime_type,'application/octet-stream'),artifact FROM report_runs WHERE id=$2 AND company_id=$1`, companyID(r), r.PathValue("id")).Scan(&filename, &mime, &artifact)
	if err != nil || len(artifact) == 0 {
		fail(w, 404, "REPORT_ARTIFACT_NOT_FOUND", "Файл отчёта не найден")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	_, _ = w.Write(artifact)
}

func reportArtifactToken(secret []byte, id, expires string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(id + "." + expires))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *api) publicReportArtifact(w http.ResponseWriter, r *http.Request) {
	expires := r.URL.Query().Get("expires")
	signature := r.URL.Query().Get("token")
	expiry, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || expiry < time.Now().Unix() || !hmac.Equal([]byte(signature), []byte(reportArtifactToken(a.jwtSecret, r.PathValue("id"), expires))) {
		fail(w, 403, "REPORT_LINK_EXPIRED", "Ссылка на отчёт недействительна или истекла")
		return
	}
	var filename, mime string
	var artifact []byte
	err = a.db.QueryRow(r.Context(), `SELECT coalesce(filename,''),coalesce(mime_type,'application/octet-stream'),artifact FROM report_runs WHERE id=$1 AND artifact IS NOT NULL`, r.PathValue("id")).Scan(&filename, &mime, &artifact)
	if err != nil {
		fail(w, 404, "REPORT_ARTIFACT_NOT_FOUND", "Файл отчёта не найден")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	_, _ = w.Write(artifact)
}

func (a *api) auditReport(r *http.Request, action, id string, data map[string]any) {
	raw, _ := json.Marshal(data)
	_, _ = a.db.Exec(r.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,entity_type,entity_id,request_id,after_data) VALUES($1,$2,$3,'report',$4,$5,$6)`, companyID(r), identity(r).Subject, action, id, r.Header.Get("X-Request-ID"), raw)
}

func queueReportRun(ctx context.Context, db *pgxpool.Pool, company, schedule, key string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `INSERT INTO report_runs(company_id,schedule_id,idempotency_key,status,format) SELECT company_id,id,$3,'queued',format FROM report_schedules WHERE company_id=$1 AND id=$2 ON CONFLICT(company_id,idempotency_key) DO UPDATE SET idempotency_key=excluded.idempotency_key RETURNING id`, company, schedule, key).Scan(&id)
	return id, err
}

func StartReportWorker(ctx context.Context, db *pgxpool.Pool, secret string) {
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			processDueReportSchedules(ctx, db)
			for processNextReportRun(ctx, db, secret) {
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func processDueReportSchedules(ctx context.Context, db *pgxpool.Pool) {
	rows, err := db.Query(ctx, `SELECT id,company_id,frequency,timezone,send_hour,send_weekday,send_monthday FROM report_schedules WHERE is_active AND next_run_at<=now() FOR UPDATE SKIP LOCKED LIMIT 50`)
	if err != nil {
		return
	}
	defer rows.Close()
	type due struct {
		id, company, frequency, timezone string
		hour                             int
		weekday, monthday                *int
	}
	items := []due{}
	for rows.Next() {
		var x due
		if rows.Scan(&x.id, &x.company, &x.frequency, &x.timezone, &x.hour, &x.weekday, &x.monthday) == nil {
			items = append(items, x)
		}
	}
	for _, x := range items {
		in := reportScheduleInput{Frequency: x.frequency, Timezone: x.timezone, SendHour: x.hour, SendWeekday: x.weekday, SendMonthday: x.monthday}
		slot := time.Now().UTC().Format("20060102T15")
		_, _ = queueReportRun(ctx, db, x.company, x.id, "scheduled:"+x.id+":"+slot)
		_, _ = db.Exec(ctx, `UPDATE report_schedules SET next_run_at=$2 WHERE id=$1`, x.id, nextReportRun(in, time.Now()))
	}
}

func processNextReportRun(ctx context.Context, db *pgxpool.Pool, secret string) bool {
	// A worker can stop after claiming a run. Recover it without creating a duplicate run.
	_, _ = db.Exec(ctx, `UPDATE report_runs SET status='queued',next_attempt_at=now(),error='Предыдущая попытка превысила допустимое время' WHERE status='processing' AND started_at<now()-interval '2 minutes' AND attempts<3`)
	_, _ = db.Exec(ctx, `UPDATE report_runs SET status='failed',error='Доставка не завершилась после трёх попыток',completed_at=now() WHERE status='processing' AND started_at<now()-interval '2 minutes' AND attempts>=3`)
	tx, err := db.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)
	var id, company, schedule, channel, format, name string
	var recipientsRaw []byte
	err = tx.QueryRow(ctx, `SELECT rr.id,rr.company_id,rr.schedule_id,rs.channel,rr.format,rs.name,rs.recipients FROM report_runs rr JOIN report_schedules rs ON rs.id=rr.schedule_id WHERE rr.status='queued' AND rr.next_attempt_at<=now() ORDER BY rr.next_attempt_at,rr.created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &company, &schedule, &channel, &format, &name, &recipientsRaw)
	if err == pgx.ErrNoRows {
		return false
	}
	if err != nil {
		return false
	}
	_, err = tx.Exec(ctx, `UPDATE report_runs SET status='processing',attempts=attempts+1,started_at=now(),error=NULL WHERE id=$1`, id)
	if err != nil {
		return false
	}
	if err = tx.Commit(ctx); err != nil {
		return false
	}
	data, err := loadReportData(ctx, db, company)
	if err != nil {
		finishReportRun(ctx, db, id, schedule, "failed", nil, "", "", err.Error())
		return true
	}
	artifact, filename, mime, err := renderReport(format, data)
	if err != nil {
		finishReportRun(ctx, db, id, schedule, "failed", nil, "", "", err.Error())
		return true
	}
	var recipients []string
	_ = json.Unmarshal(recipientsRaw, &recipients)
	status, deliveryError := deliverReport(ctx, db, company, id, secret, channel, recipients, name, data, artifact, filename, mime)
	finishReportRun(ctx, db, id, schedule, status, artifact, filename, mime, deliveryError)
	return true
}

func loadReportData(ctx context.Context, db *pgxpool.Pool, company string) (reportData, error) {
	d := reportData{Period: "последние 30 дней"}
	err := db.QueryRow(ctx, `SELECT name FROM companies WHERE id=$1`, company).Scan(&d.Company)
	if err != nil {
		return d, err
	}
	err = db.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE coalesce(activity.last_visit,c.created_at)>=now()-interval '30 days'),count(*) FILTER(WHERE c.total_visits>=2),count(*) FILTER(WHERE coalesce(activity.last_visit,c.created_at)<now()-interval '45 days') FROM customers c LEFT JOIN LATERAL (SELECT max(v.created_at) last_visit FROM visits v WHERE v.company_id=c.company_id AND v.customer_id=c.id) activity ON true WHERE c.company_id=$1 AND c.deleted_at IS NULL`, company).Scan(&d.Customers, &d.ActiveCustomers, &d.RepeatCustomers, &d.AtRisk)
	if err != nil {
		return d, err
	}
	_ = db.QueryRow(ctx, `SELECT count(*),coalesce(sum(net_amount),0),coalesce(sum(net_amount) FILTER(WHERE customer_id IS NOT NULL),0) FROM sales_transactions WHERE company_id=$1 AND status='completed' AND occurred_at>=now()-interval '30 days'`, company).Scan(&d.Transactions, &d.Revenue, &d.MemberRevenue)
	_ = db.QueryRow(ctx, `SELECT coalesce(sum((remaining_amount::numeric/issued_amount)*monetary_value),0) FROM bonus_lots WHERE company_id=$1 AND status IN('pending','active') AND remaining_amount>0 AND (expires_at IS NULL OR expires_at>now())`, company).Scan(&d.BonusLiability)
	_ = db.QueryRow(ctx, `SELECT coalesce(sum(conversion_value) FILTER(WHERE conversion_type='purchased'),0) FROM campaign_conversions WHERE company_id=$1 AND occurred_at>=now()-interval '30 days'`, company).Scan(&d.CampaignRevenue)
	return d, nil
}

func reportRows(d reportData) [][]string {
	return [][]string{{"Показатель", "Значение"}, {"Компания", d.Company}, {"Период", d.Period}, {"Клиенты", strconv.Itoa(d.Customers)}, {"Активные клиенты", strconv.Itoa(d.ActiveCustomers)}, {"Повторные клиенты", strconv.Itoa(d.RepeatCustomers)}, {"Потерянные клиенты", strconv.Itoa(d.AtRisk)}, {"Закрытые чеки", strconv.Itoa(d.Transactions)}, {"Выручка", fmt.Sprintf("%.2f KZT", d.Revenue)}, {"Выручка участников", fmt.Sprintf("%.2f KZT", d.MemberRevenue)}, {"Обязательства по бонусам", fmt.Sprintf("%.2f KZT", d.BonusLiability)}, {"Выручка кампаний", fmt.Sprintf("%.2f KZT", d.CampaignRevenue)}}
}

func renderReport(format string, d reportData) ([]byte, string, string, error) {
	stamp := time.Now().UTC().Format("2006-01-02")
	switch format {
	case "summary":
		body := []byte(reportPlainText(d))
		return body, "tappix-report-" + stamp + ".txt", "text/plain; charset=utf-8", nil
	case "csv":
		var b bytes.Buffer
		b.WriteString("\xEF\xBB\xBF")
		w := csv.NewWriter(&b)
		_ = w.WriteAll(reportRows(d))
		w.Flush()
		return b.Bytes(), "tappix-report-" + stamp + ".csv", "text/csv; charset=utf-8", w.Error()
	case "xlsx":
		b, err := renderXLSX(reportRows(d))
		return b, "tappix-report-" + stamp + ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", err
	case "pdf":
		return renderPDF(d), "tappix-report-" + stamp + ".pdf", "application/pdf", nil
	}
	return nil, "", "", fmt.Errorf("unsupported report format")
}

func reportPlainText(d reportData) string {
	return fmt.Sprintf("Tappix — отчёт владельца\n%s · %s\n\nКлиенты: %d\nАктивные: %d\nПовторные: %d\nПотерянные: %d\nЧеки: %d\nВыручка: %.2f KZT\nВыручка участников: %.2f KZT\nОбязательства по бонусам: %.2f KZT\nВыручка кампаний: %.2f KZT\n", d.Company, d.Period, d.Customers, d.ActiveCustomers, d.RepeatCustomers, d.AtRisk, d.Transactions, d.Revenue, d.MemberRevenue, d.BonusLiability, d.CampaignRevenue)
}

func renderXLSX(rows [][]string) ([]byte, error) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	files := map[string]string{"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`, "_rels/.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`, "xl/workbook.xml": `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Tappix" sheetId="1" r:id="rId1"/></sheets></workbook>`, "xl/_rels/workbook.xml.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`}
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, row := range rows {
		sheet.WriteString(fmt.Sprintf(`<row r="%d">`, ri+1))
		for ci, value := range row {
			cell := string(rune('A'+ci)) + strconv.Itoa(ri+1)
			sheet.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, cell, html.EscapeString(value)))
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	files["xl/worksheets/sheet1.xml"] = sheet.String()
	for name, content := range files {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func renderPDF(d reportData) []byte {
	text := strings.ReplaceAll(reportPlainText(d), "(", "\\(")
	text = strings.ReplaceAll(text, ")", "\\)")
	text = strings.ReplaceAll(text, "\n", ") Tj T* (")
	stream := "BT /F1 10 Tf 50 790 Td 13 TL (" + text + ") Tj ET"
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream), "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, xref)
	return b.Bytes()
}

func deliverReport(ctx context.Context, db *pgxpool.Pool, company, runID, secret, channel string, recipients []string, subject string, data reportData, artifact []byte, filename, mime string) (string, string) {
	if len(recipients) == 0 {
		return "skipped", "Получатели не указаны"
	}
	switch channel {
	case "email":
		for _, recipient := range recipients {
			if !strings.Contains(recipient, "@") {
				return "failed", "Некорректный email"
			}
			if err := sendEmailAttachment(ctx, recipient, "Tappix: "+subject, reportPlainText(data), artifact, filename, mime); err != nil {
				return "failed", err.Error()
			}
		}
		return "sent", ""
	case "whatsapp":
		appURL := strings.TrimRight(os.Getenv("APP_URL"), "/")
		token := os.Getenv("WHATSAPP_ACCESS_TOKEN")
		phoneID := os.Getenv("WHATSAPP_PHONE_NUMBER_ID")
		if !strings.HasPrefix(appURL, "https://") || token == "" || phoneID == "" {
			return "skipped", "Подключите WhatsApp и публичный HTTPS-адрес Tappix"
		}
		expires := strconv.FormatInt(time.Now().Add(7*24*time.Hour).Unix(), 10)
		link := fmt.Sprintf("%s/api/v1/public/reports/%s?expires=%s&token=%s", appURL, runID, expires, reportArtifactToken([]byte(secret), runID, expires))
		for _, recipient := range recipients {
			phone := nonDigits.ReplaceAllString(recipient, "")
			if len(phone) < 10 {
				return "failed", "Некорректный номер WhatsApp"
			}
			payload, _ := json.Marshal(map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": phone, "type": "document", "document": map[string]string{"link": link, "filename": filename, "caption": "Tappix: " + subject}})
			url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", envValue("WHATSAPP_GRAPH_VERSION", "v23.0"), phoneID)
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
			if err != nil {
				return "failed", err.Error()
			}
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set("Content-Type", "application/json")
			response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
			if err != nil {
				return "failed", err.Error()
			}
			detail, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
			response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				return "failed", fmt.Sprintf("WhatsApp отклонил доставку: HTTP %d (%s)", response.StatusCode, strings.TrimSpace(string(detail)))
			}
		}
		return "sent", ""
	case "webhook":
		for _, endpointID := range recipients {
			var target string
			var encrypted []byte
			err := db.QueryRow(ctx, `SELECT url,encrypted_secret FROM webhook_endpoints WHERE id=$2 AND company_id=$1 AND status='active'`, company, endpointID).Scan(&target, &encrypted)
			if err != nil {
				return "failed", "Webhook endpoint не найден или отключён"
			}
			if err = validateOutboundURL(ctx, target); err != nil {
				return "failed", "Webhook endpoint недоступен"
			}
			webhookSecret, err := decryptIntegrationSecret(integrationEncryptionKey(secret), encrypted)
			if err != nil {
				return "failed", "Не удалось открыть секрет webhook"
			}
			payload, _ := json.Marshal(map[string]any{"event": "report.ready", "reportRunId": runID, "name": subject, "filename": filename, "mimeType": mime, "summary": reportPlainText(data)})
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			mac := hmac.New(sha256.New, webhookSecret)
			_, _ = mac.Write([]byte(timestamp + "."))
			_, _ = mac.Write(payload)
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
			if err != nil {
				return "failed", err.Error()
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("User-Agent", "Tappix-Reports/1.0")
			request.Header.Set("X-Tappix-Event", "report.ready")
			request.Header.Set("X-Tappix-Event-ID", runID)
			request.Header.Set("X-Tappix-Timestamp", timestamp)
			request.Header.Set("X-Tappix-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
			response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
			if err != nil {
				return "failed", err.Error()
			}
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				return "failed", fmt.Sprintf("Webhook вернул HTTP %d", response.StatusCode)
			}
		}
		return "sent", ""
	}
	return "failed", "Неизвестный канал"
}

func sendEmailAttachment(ctx context.Context, recipient, subject, body string, artifact []byte, filename, mime string) error {
	host := envValue("SMTP_HOST", "mailpit")
	deliveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(deliveryCtx, "tcp", net.JoinHostPort(host, envValue("SMTP_PORT", "1025")))
	if err != nil {
		return fmt.Errorf("smtp connection: %w", err)
	}
	defer connection.Close()
	if deadline, ok := deliveryCtx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if err = configureSMTPClient(client, host); err != nil {
		return err
	}
	from := fromAddress(envValue("SMTP_FROM", "Tappix <noreply@tappix.kz>"))
	if err = client.Mail(from); err != nil {
		return err
	}
	if err = client.Rcpt(recipient); err != nil {
		return err
	}
	dataWriter, err := client.Data()
	if err != nil {
		return err
	}
	var message bytes.Buffer
	boundary := "tappix-report-boundary"
	fmt.Fprintf(&message, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%q\r\n\r\n", envValue("SMTP_FROM", "Tappix <noreply@tappix.kz>"), recipient, subject, boundary)
	multi := multipart.NewWriter(&message)
	_ = multi.SetBoundary(boundary)
	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	part, _ := multi.CreatePart(textHeader)
	_, _ = part.Write([]byte(body))
	fileHeader := textproto.MIMEHeader{}
	fileHeader.Set("Content-Type", mime)
	fileHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	fileHeader.Set("Content-Transfer-Encoding", "base64")
	part, _ = multi.CreatePart(fileHeader)
	encoded := base64.StdEncoding.EncodeToString(artifact)
	for len(encoded) > 76 {
		_, _ = fmt.Fprintf(part, "%s\r\n", encoded[:76])
		encoded = encoded[76:]
	}
	_, _ = fmt.Fprintf(part, "%s", encoded)
	_ = multi.Close()
	if _, err = dataWriter.Write(message.Bytes()); err != nil {
		return err
	}
	if err = dataWriter.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func finishReportRun(ctx context.Context, db *pgxpool.Pool, id, schedule, status string, artifact []byte, filename, mime, errorText string) {
	if status == "failed" {
		var attempts int
		if db.QueryRow(ctx, `SELECT attempts FROM report_runs WHERE id=$1`, id).Scan(&attempts) == nil && attempts < 3 {
			delay := reportRetryDelay(attempts)
			_, retryErr := db.Exec(ctx, `UPDATE report_runs SET status='queued',artifact=$2,filename=nullif($3,''),mime_type=nullif($4,''),error=nullif($5,''),completed_at=NULL,next_attempt_at=now()+make_interval(secs=>$6) WHERE id=$1`, id, artifact, filename, mime, errorText, int(delay.Seconds()))
			if retryErr == nil {
				_, _ = db.Exec(ctx, `UPDATE report_schedules SET last_run_at=now(),last_status='queued',last_error=nullif($2,''),updated_at=now() WHERE id=$1`, schedule, errorText)
				return
			}
		}
	}
	_, err := db.Exec(ctx, `UPDATE report_runs SET status=$2,artifact=$3,filename=nullif($4,''),mime_type=nullif($5,''),error=nullif($6,''),completed_at=now() WHERE id=$1`, id, status, artifact, filename, mime, errorText)
	if err != nil {
		slog.Error("report run update failed", "report_run_id", id, "error", err)
		return
	}
	_, _ = db.Exec(ctx, `UPDATE report_schedules SET last_run_at=now(),last_status=$2,last_error=nullif($3,''),updated_at=now() WHERE id=$1`, schedule, status, errorText)
}

func reportRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	return time.Duration(attempts) * time.Minute
}
