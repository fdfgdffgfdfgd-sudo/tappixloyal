package httpapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

func StartAutomation(ctx context.Context, db *pgxpool.Pool) {
	run := func() {
		if _, err := processBirthdayBonuses(ctx, db, ""); err != nil {
			slog.Error("birthday automation worker failed", "error", err)
		}
		if _, err := processCampaignAutomations(ctx, db, ""); err != nil {
			slog.Error("campaign automation worker failed", "error", err)
		}
		if _, err := processReferralRewards(ctx, db, ""); err != nil {
			slog.Error("referral reward worker failed", "error", err)
		}
	}
	go func() {
		run()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func processReferralRewards(ctx context.Context, db *pgxpool.Pool, onlyCompany string) (int, error) {
	rows, err := db.Query(ctx, `SELECT id,company_id,attribution_id,beneficiary_customer_id,reward_value::integer,idempotency_key FROM referral_rewards
		WHERE status='pending' AND available_at<=now() AND ($1='' OR company_id::text=$1) ORDER BY available_at LIMIT 200`, onlyCompany)
	if err != nil {
		return 0, err
	}
	type candidate struct {
		id, company, attribution, customer, key string
		amount                                  int
	}
	items := []candidate{}
	for rows.Next() {
		var x candidate
		if rows.Scan(&x.id, &x.company, &x.attribution, &x.customer, &x.amount, &x.key) == nil {
			items = append(items, x)
		}
	}
	rows.Close()
	issued := 0
	for _, x := range items {
		tx, beginErr := db.Begin(ctx)
		if beginErr != nil {
			continue
		}
		var balance int
		err = tx.QueryRow(ctx, `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, x.company, x.customer).Scan(&balance)
		if err == nil {
			var ledgerID string
			newLedger := false
			err = tx.QueryRow(ctx, `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description,idempotency_key)
				VALUES($1,$2,'credit',$3,$4,'Награда за рекомендацию',$5) ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING RETURNING id`, x.company, x.customer, x.amount, balance+x.amount, x.key).Scan(&ledgerID)
			if errors.Is(err, pgx.ErrNoRows) {
				err = nil
			} else if err == nil {
				newLedger = true
				err = posintegration.IssueBonusLot(ctx, tx, x.company, x.customer, ledgerID, "", x.amount)
			}
			if err == nil && newLedger {
				_, err = tx.Exec(ctx, `UPDATE customers SET total_points=total_points+$3,updated_at=now() WHERE company_id=$1 AND id=$2`, x.company, x.customer, x.amount)
			}
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE referral_rewards SET status='issued',issued_at=now(),bonus_ledger_id=nullif($3,'')::uuid WHERE company_id=$1 AND id=$2`, x.company, x.id, ledgerID)
			}
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE referral_attributions SET status='rewarded',updated_at=now() WHERE company_id=$1 AND id=$2 AND NOT EXISTS(SELECT 1 FROM referral_rewards WHERE attribution_id=$2 AND status='pending')`, x.company, x.attribution)
			}
		}
		if err == nil && tx.Commit(ctx) == nil {
			issued++
		} else {
			_ = tx.Rollback(ctx)
		}
	}
	return issued, nil
}

func processBirthdayBonuses(ctx context.Context, db *pgxpool.Pool, onlyCompany string) (int, error) {
	query := `SELECT c.id,c.company_id,coalesce((lr.actions->>'amount')::integer,(ca.settings->>'bonusAmount')::integer,0) FROM customers c JOIN companies co ON co.id=c.company_id AND co.status='active' JOIN campaign_automations ca ON ca.company_id=c.company_id AND ca.trigger_type='birthday_bonus' AND ca.is_active LEFT JOIN loyalty_rules lr ON lr.company_id=c.company_id AND lr.event_type='customer_birthday' AND lr.is_active WHERE c.deleted_at IS NULL AND c.birthday IS NOT NULL AND extract(month from c.birthday)=extract(month from current_date) AND extract(day from c.birthday)=extract(day from current_date) AND ($1='' OR c.company_id::text=$1)`
	rows, err := db.Query(ctx, query, onlyCompany)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type item struct {
		id, company string
		amount      int
	}
	items := []item{}
	for rows.Next() {
		var x item
		if rows.Scan(&x.id, &x.company, &x.amount) == nil && x.amount > 0 {
			items = append(items, x)
		}
	}
	processed := 0
	year := time.Now().UTC().Year()
	for _, x := range items {
		tx, e := db.Begin(ctx)
		if e != nil {
			slog.Warn("birthday automation transaction failed", "error", e)
			continue
		}
		key := fmt.Sprintf("birthday:%s:%d", x.id, year)
		var eventID string
		e = tx.QueryRow(ctx, `INSERT INTO loyalty_events(company_id,customer_id,event_type,event_key,payload) VALUES($1,$2,'customer_birthday',$3,jsonb_build_object('amount',$4::integer)) ON CONFLICT(company_id,event_key) DO NOTHING RETURNING id`, x.company, x.id, key, x.amount).Scan(&eventID)
		if errors.Is(e, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			continue
		}
		if e != nil {
			slog.Warn("birthday automation event failed", "customer", x.id, "error", e)
			_ = tx.Rollback(ctx)
			continue
		}
		var balance int
		e = tx.QueryRow(ctx, `UPDATE customers SET total_points=total_points+$3,updated_at=now() WHERE company_id=$1 AND id=$2 RETURNING total_points`, x.company, x.id, x.amount).Scan(&balance)
		if e == nil {
			var ledgerID string
			e = tx.QueryRow(ctx, `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description,idempotency_key) VALUES($1,$2,'credit',$3,$4,'Бонус на день рождения',$5) RETURNING id`, x.company, x.id, x.amount, balance, key).Scan(&ledgerID)
			if e == nil {
				e = posintegration.IssueBonusLot(ctx, tx, x.company, x.id, ledgerID, "", x.amount)
			}
		}
		if e == nil && tx.Commit(ctx) == nil {
			processed++
		} else {
			slog.Warn("birthday automation credit failed", "customer", x.id, "error", e)
			_ = tx.Rollback(ctx)
		}
	}
	return processed, nil
}

type automationCandidate struct {
	automationID, companyID, customerID, triggerType, triggerKey string
	name, email, channel, subject, message                       string
	amount                                                       int
}

func processCampaignAutomations(ctx context.Context, db *pgxpool.Pool, onlyCompany string) (int, error) {
	rows, err := db.Query(ctx, `WITH last_activity AS (
		SELECT c.id customer_id,c.company_id,greatest(coalesce(max(t.occurred_at),c.created_at),coalesce(max(v.created_at),c.created_at)) last_at
		FROM customers c LEFT JOIN sales_transactions t ON t.company_id=c.company_id AND t.customer_id=c.id AND t.original_transaction_id IS NULL AND t.status IN('completed','partially_refunded','refunded') AND NOT t.sandbox
		LEFT JOIN visits v ON v.company_id=c.company_id AND v.customer_id=c.id AND v.reversed_at IS NULL WHERE c.deleted_at IS NULL GROUP BY c.id
	), candidates AS (
		SELECT a.id automation_id,a.company_id,c.id customer_id,a.trigger_type,'automation:birthday:'||c.id||':'||extract(year from current_date)::integer trigger_key,c.first_name,c.email,a.channel,a.subject,a.message,coalesce((a.settings->>'bonusAmount')::integer,0) amount
		FROM campaign_automations a JOIN customers c ON c.company_id=a.company_id AND c.deleted_at IS NULL
		WHERE a.is_active AND a.trigger_type='birthday_bonus' AND c.birthday IS NOT NULL AND extract(month from c.birthday)=extract(month from current_date) AND extract(day from c.birthday)=extract(day from current_date)
		UNION ALL
		SELECT a.id,a.company_id,c.id,a.trigger_type,'automation:expiry:'||c.id||':'||current_date,c.first_name,c.email,a.channel,a.subject,a.message,sum(l.remaining_amount)::integer
		FROM campaign_automations a JOIN customers c ON c.company_id=a.company_id AND c.deleted_at IS NULL JOIN bonus_lots l ON l.company_id=c.company_id AND l.customer_id=c.id
		WHERE a.is_active AND a.trigger_type='bonus_expiry_3d' AND l.status IN('pending','active') AND l.remaining_amount>0 AND l.expires_at::date=current_date+coalesce((a.settings->>'daysBefore')::integer,3)
		GROUP BY a.id,c.id
		UNION ALL
		SELECT a.id,a.company_id,c.id,a.trigger_type,'automation:winback:'||c.id||':'||la.last_at::date,c.first_name,c.email,a.channel,a.subject,a.message,0
		FROM campaign_automations a JOIN customers c ON c.company_id=a.company_id AND c.deleted_at IS NULL JOIN last_activity la ON la.company_id=c.company_id AND la.customer_id=c.id
		WHERE a.is_active AND a.trigger_type='winback_30d' AND la.last_at<=now()-make_interval(days=>coalesce((a.settings->>'inactiveDays')::integer,30))
		UNION ALL
		SELECT a.id,a.company_id,c.id,a.trigger_type,'automation:event:'||e.id,c.first_name,c.email,a.channel,a.subject,a.message,0
		FROM campaign_automations a JOIN customer_events e ON e.company_id=a.company_id
		JOIN customers c ON c.company_id=e.company_id AND c.id=e.customer_id AND c.deleted_at IS NULL
		WHERE a.is_active AND ((a.trigger_type='near_reward' AND e.event_type='reward.almost_unlocked')
			OR (a.trigger_type='reward_unlocked' AND e.event_type='reward.unlocked')
			OR (a.trigger_type='nfc_registration' AND e.event_type='customer.registered'))
	) SELECT automation_id,company_id,customer_id,trigger_type,trigger_key,first_name,coalesce(email,''),channel,subject,message,amount FROM candidates WHERE ($1='' OR company_id::text=$1)`, onlyCompany)
	if err != nil {
		return 0, err
	}
	items := []automationCandidate{}
	for rows.Next() {
		var item automationCandidate
		if rows.Scan(&item.automationID, &item.companyID, &item.customerID, &item.triggerType, &item.triggerKey, &item.name, &item.email, &item.channel, &item.subject, &item.message, &item.amount) == nil {
			items = append(items, item)
		}
	}
	rows.Close()
	processed := 0
	for _, item := range items {
		status, errorText := "sent", ""
		message := strings.ReplaceAll(strings.ReplaceAll(item.message, "{{name}}", item.name), "{{amount}}", fmt.Sprintf("%d", item.amount))
		var runID string
		var attempt int
		err = db.QueryRow(ctx, `INSERT INTO campaign_automation_runs(company_id,automation_id,customer_id,trigger_key,status,channel,recipient,payload,attempt_count)
			VALUES($1,$2,$3,$4,'pending',$5,nullif($6,''),jsonb_build_object('amount',$7::integer),1)
			ON CONFLICT(company_id,trigger_key) DO UPDATE SET
				status='pending',attempt_count=campaign_automation_runs.attempt_count+1,
				next_attempt_at=null,error=null,updated_at=now()
			WHERE (campaign_automation_runs.status='pending' AND campaign_automation_runs.updated_at<now()-interval '5 minutes')
				OR (campaign_automation_runs.status='failed' AND campaign_automation_runs.attempt_count<3 AND campaign_automation_runs.next_attempt_at<=now())
			RETURNING id,attempt_count`, item.companyID, item.automationID, item.customerID, item.triggerKey, item.channel, item.email, item.amount).Scan(&runID, &attempt)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return processed, err
		}
		if item.channel != "email" || item.email == "" {
			status, errorText = "skipped", "Канал недоступен или у клиента нет email"
		} else if sendErr := sendEmailWithTimeout(ctx, item.email, item.subject, message); sendErr != nil {
			status, errorText = "failed", sendErr.Error()
		}
		var nextAttempt *time.Time
		if status == "failed" && attempt < 3 {
			retryAt := time.Now().Add(time.Duration(attempt*attempt) * time.Minute)
			nextAttempt = &retryAt
		}
		_, err = db.Exec(ctx, `UPDATE campaign_automation_runs SET status=$2::varchar,error=nullif($3::text,''),sent_at=CASE WHEN $2::varchar='sent' THEN now() ELSE NULL END,next_attempt_at=$4::timestamptz,updated_at=now() WHERE id=$1`, runID, status, errorText, nextAttempt)
		if err != nil {
			return processed, err
		}
		if status == "sent" {
			_, err = db.Exec(ctx, `INSERT INTO customer_events(company_id,customer_id,event_type,source,properties,idempotency_key)
				VALUES($1,$2,'campaign.sent','automation',jsonb_build_object('automationId',$3::text,'triggerType',$4::text),'campaign-sent:'||$5::text)
				ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, item.companyID, item.customerID, item.automationID, item.triggerType, runID)
			if err != nil {
				return processed, err
			}
			processed++
		}
	}
	return processed, nil
}

func sendEmailWithTimeout(ctx context.Context, recipient, subject, message string) error {
	host := envValue("SMTP_HOST", "mailpit")
	address := net.JoinHostPort(host, envValue("SMTP_PORT", "1025"))
	deliveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(deliveryCtx, "tcp", address)
	if err != nil {
		return fmt.Errorf("smtp connection: %w", err)
	}
	defer connection.Close()
	if deadline, ok := deliveryCtx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()
	if err = configureSMTPClient(client, host); err != nil {
		return err
	}
	from := fromAddress(envValue("SMTP_FROM", "Tappix <noreply@tappix.kz>"))
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("smtp sender: %w", err)
	}
	if err = client.Rcpt(recipient); err != nil {
		return fmt.Errorf("smtp recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	payload := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", envValue("SMTP_FROM", "Tappix <noreply@tappix.kz>"), recipient, subject, message)
	_, writeErr := writer.Write([]byte(payload))
	closeErr := writer.Close()
	if writeErr != nil {
		return fmt.Errorf("smtp write: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("smtp finish: %w", closeErr)
	}
	if err = client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func configureSMTPClient(client *smtp.Client, host string) error {
	requireTLS := strings.EqualFold(envValue("SMTP_TLS", "false"), "true")
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp tls: %w", err)
		}
	} else if requireTLS {
		return fmt.Errorf("smtp tls: server does not offer STARTTLS")
	}
	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	if username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, os.Getenv("SMTP_PASSWORD"), host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	return nil
}

func (a *api) processBirthdays(w http.ResponseWriter, r *http.Request) {
	count, err := processBirthdayBonuses(r.Context(), a.db, companyID(r))
	if err != nil {
		fail(w, 500, "AUTOMATION_FAILED", "Не удалось обработать birthday-бонусы")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]int{"processed": count}})
}

func (a *api) inactiveCustomers(w http.ResponseWriter, r *http.Request) {
	days := clamp(parseInt(r.URL.Query().Get("days"), 30), 1, 3650)
	rows, err := a.db.Query(r.Context(), `SELECT c.id,c.first_name,c.last_name,c.phone,c.total_points,c.total_visits,max(v.created_at) FROM customers c LEFT JOIN visits v ON v.company_id=c.company_id AND v.customer_id=c.id WHERE c.company_id=$1 AND c.deleted_at IS NULL GROUP BY c.id HAVING coalesce(max(v.created_at),c.created_at)<now()-make_interval(days=>$2::integer) ORDER BY max(v.created_at) NULLS FIRST LIMIT 500`, companyID(r), days)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось найти неактивных клиентов")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, first, last, phone string
		var points, visits int
		var lastVisit *time.Time
		if rows.Scan(&id, &first, &last, &phone, &points, &visits, &lastVisit) == nil {
			items = append(items, map[string]any{"id": id, "firstName": first, "lastName": last, "phone": phone, "points": points, "visits": visits, "lastVisitAt": lastVisit})
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"days": days, "items": items, "total": len(items)}})
}
