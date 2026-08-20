package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var nonPhoneDigits = regexp.MustCompile(`\D`)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func NormalizePhone(value string) string {
	digits := nonPhoneDigits.ReplaceAllString(value, "")
	if len(digits) == 11 && digits[0] == '8' {
		digits = "7" + digits[1:]
	}
	if len(digits) == 10 {
		digits = "7" + digits
	}
	if len(digits) < 10 || len(digits) > 15 {
		return ""
	}
	return "+" + digits
}

func validateTransaction(in CanonicalTransaction) error {
	if in.CompanyID == "" || in.ConnectionID == "" || strings.TrimSpace(in.Provider) == "" || strings.TrimSpace(in.ExternalID) == "" {
		return errors.New("company, connection, provider and external id are required")
	}
	if in.OccurredAt.IsZero() {
		return errors.New("occurredAt is required")
	}
	amounts := []float64{in.GrossAmount, in.DiscountAmount, in.BonusPaidAmount, in.CashPaidAmount, in.NetAmount, float64(in.BonusEarned), float64(in.BonusSpent)}
	for _, amount := range amounts {
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
			return errors.New("amounts must be finite and non-negative")
		}
	}
	if in.NetAmount > in.GrossAmount || in.DiscountAmount > in.GrossAmount {
		return errors.New("transaction totals are inconsistent")
	}
	if in.Status != "completed" && in.Status != "refunded" && in.Status != "cancelled" && in.Status != "partially_refunded" && in.Status != "pending" {
		return errors.New("unsupported transaction status")
	}
	if in.Currency == "" {
		in.Currency = "KZT"
	}
	for _, item := range in.Items {
		if strings.TrimSpace(item.Name) == "" || item.Quantity <= 0 || math.IsNaN(item.Quantity) || math.IsInf(item.Quantity, 0) || math.IsNaN(item.UnitPrice) || math.IsInf(item.UnitPrice, 0) || math.IsNaN(item.NetAmount) || math.IsInf(item.NetAmount, 0) || item.UnitPrice < 0 || item.NetAmount < 0 || (item.CostAmount != nil && (*item.CostAmount < 0 || math.IsNaN(*item.CostAmount) || math.IsInf(*item.CostAmount, 0))) {
			return errors.New("invalid transaction item")
		}
	}
	for _, payment := range in.Payments {
		if payment.Amount < 0 || math.IsNaN(payment.Amount) || math.IsInf(payment.Amount, 0) || payment.OccurredAt.IsZero() {
			return errors.New("invalid payment")
		}
		if payment.Type != "cash" && payment.Type != "card" && payment.Type != "wallet" && payment.Type != "bank_transfer" {
			return errors.New("unsupported payment type")
		}
		if payment.Status != "pending" && payment.Status != "authorized" && payment.Status != "captured" && payment.Status != "refunded" && payment.Status != "failed" {
			return errors.New("unsupported payment status")
		}
	}
	return nil
}

func (s *Service) Ingest(ctx context.Context, in CanonicalTransaction) (IngestResult, error) {
	if err := validateTransaction(in); err != nil {
		return IngestResult{}, err
	}
	if in.Currency == "" {
		in.Currency = "KZT"
	}
	if in.Source == "" {
		in.Source = "pos"
	}
	if len(in.RawPayload) == 0 {
		in.RawPayload = json.RawMessage(`{}`)
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return IngestResult{}, err
	}
	defer tx.Rollback(ctx)
	var connectionValid bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM integration_connections WHERE id=$1 AND company_id=$2 AND deleted_at IS NULL AND status<>'disabled')`, in.ConnectionID, in.CompanyID).Scan(&connectionValid); err != nil {
		return IngestResult{}, err
	}
	if !connectionValid {
		return IngestResult{}, errors.New("integration connection does not belong to tenant")
	}

	var existingID, existingCustomer string
	err = tx.QueryRow(ctx, `SELECT id::text,coalesce(customer_id::text,'') FROM sales_transactions WHERE company_id=$1 AND provider=$2 AND external_id=$3`, in.CompanyID, in.Provider, in.ExternalID).Scan(&existingID, &existingCustomer)
	if err == nil {
		return IngestResult{TransactionID: existingID, CustomerID: existingCustomer, Duplicate: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return IngestResult{}, err
	}

	branchID, err := resolveBranch(ctx, tx, in.CompanyID, in.ConnectionID, in.BranchID, in.ExternalLocationID)
	if err != nil {
		return IngestResult{}, err
	}
	customerID, err := resolveCustomer(ctx, tx, in.CompanyID, in.ConnectionID, in.CustomerID, in.ExternalCustomerID, in.CustomerPhone)
	if err != nil {
		return IngestResult{}, err
	}

	var originalID any
	if in.OriginalExternalID != "" {
		var id string
		if e := tx.QueryRow(ctx, `SELECT id FROM sales_transactions WHERE company_id=$1 AND provider=$2 AND external_id=$3`, in.CompanyID, in.Provider, in.OriginalExternalID).Scan(&id); e == nil {
			originalID = id
		}
	}
	var campaign any
	if in.CampaignID != "" {
		campaign = in.CampaignID
	}
	metadata, _ := json.Marshal(in.Metadata)
	var transactionID string
	err = tx.QueryRow(ctx, `INSERT INTO sales_transactions(
		company_id,branch_id,customer_id,integration_connection_id,provider,external_id,status,occurred_at,
		gross_amount,discount_amount,bonus_paid_amount,cash_paid_amount,net_amount,cost_amount,currency,
		receipt_number,source,campaign_id,original_transaction_id,idempotency_key,raw_payload,metadata,sandbox
	) VALUES($1,nullif($2,'')::uuid,nullif($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,upper($15),$16,$17,$18,$19,$20,$21,$22,$23)
	RETURNING id`, in.CompanyID, branchID, customerID, in.ConnectionID, in.Provider, in.ExternalID, in.Status, in.OccurredAt,
		in.GrossAmount, in.DiscountAmount, in.BonusPaidAmount, in.CashPaidAmount, in.NetAmount, in.CostAmount, in.Currency,
		in.ReceiptNumber, in.Source, campaign, originalID, "pos:"+in.Provider+":"+in.ExternalID, in.RawPayload, metadata, in.Sandbox).Scan(&transactionID)
	if err != nil {
		return IngestResult{}, err
	}

	for _, item := range in.Items {
		itemMeta, _ := json.Marshal(item.Metadata)
		_, err = tx.Exec(ctx, `INSERT INTO sales_transaction_items(company_id,transaction_id,external_item_id,product_external_id,sku,name,category,quantity,unit_price,gross_amount,discount_amount,net_amount,cost_amount,metadata)
			VALUES($1,$2,nullif($3,''),nullif($4,''),nullif($5,''),$6,nullif($7,''),$8,$9,$10,$11,$12,$13,$14)`, in.CompanyID, transactionID, item.ExternalID, item.ProductExternalID, item.SKU, item.Name, item.Category, item.Quantity, item.UnitPrice, item.GrossAmount, item.DiscountAmount, item.NetAmount, item.CostAmount, itemMeta)
		if err != nil {
			return IngestResult{}, err
		}
	}
	for _, payment := range in.Payments {
		paymentMeta, _ := json.Marshal(payment.Metadata)
		_, err = tx.Exec(ctx, `INSERT INTO payments(company_id,transaction_id,provider,external_id,payment_type,status,amount,currency,occurred_at,metadata)
			VALUES($1,$2,nullif($3,''),nullif($4,''),$5,$6,$7,upper($8),$9,$10)`, in.CompanyID, transactionID, payment.Provider, payment.ExternalID, payment.Type, payment.Status, payment.Amount, in.Currency, payment.OccurredAt, paymentMeta)
		if err != nil {
			return IngestResult{}, err
		}
	}

	result := IngestResult{TransactionID: transactionID, CustomerID: customerID}
	if customerID != "" && in.Status == "completed" && !in.Sandbox {
		if err = applyCampaignAttribution(ctx, tx, &in, transactionID, customerID); err != nil {
			return IngestResult{}, err
		}
		visitID, e := applyLoyalty(ctx, tx, in, transactionID, branchID, customerID)
		if e != nil {
			return IngestResult{}, e
		}
		result.VisitID = visitID
		if e = appendPurchaseEvent(ctx, tx, in.CompanyID, customerID, transactionID, branchID, in); e != nil {
			return IngestResult{}, e
		}
		if e = applyReferralQualification(ctx, tx, in.CompanyID, customerID, transactionID, in.NetAmount); e != nil {
			return IngestResult{}, e
		}
	}
	if !in.Sandbox {
		if err = appendOutbox(ctx, tx, in, transactionID, customerID, branchID); err != nil {
			return IngestResult{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO integration_jobs(company_id,connection_id,job_type,resource,idempotency_key,payload)
			VALUES($1,$2,'analytics_projection','sales_transaction',$3,jsonb_build_object('transactionId',$4::text)) ON CONFLICT DO NOTHING`, in.CompanyID, in.ConnectionID, "analytics:"+transactionID, transactionID)
		if err != nil {
			return IngestResult{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return IngestResult{}, err
	}
	return result, nil
}

func applyReferralQualification(ctx context.Context, tx pgx.Tx, companyID, customerID, transactionID string, netAmount float64) error {
	var attributionID, programID, referrerID string
	var referrerReward, friendReward float64
	var delayDays int
	err := tx.QueryRow(ctx, `SELECT a.id,p.id,a.referrer_customer_id,p.referrer_reward_value,p.friend_reward_value,p.reward_delay_days
		FROM referral_attributions a JOIN referral_programs p ON p.id=a.program_id AND p.company_id=a.company_id AND p.status='active'
		WHERE a.company_id=$1 AND a.referred_customer_id=$2 AND a.status='registered' AND $3>=p.minimum_purchase_amount
		AND (p.max_rewards_per_customer IS NULL OR (SELECT count(*) FROM referral_attributions x WHERE x.company_id=a.company_id AND x.program_id=a.program_id AND x.referrer_customer_id=a.referrer_customer_id AND x.status IN('qualified','reward_pending','rewarded'))<p.max_rewards_per_customer)
		AND (p.max_rewards_per_month IS NULL OR (SELECT count(*) FROM referral_attributions x WHERE x.company_id=a.company_id AND x.program_id=a.program_id AND x.status IN('qualified','reward_pending','rewarded') AND x.qualified_at>=date_trunc('month',now()))<p.max_rewards_per_month)
		ORDER BY a.registered_at LIMIT 1 FOR UPDATE OF a`, companyID, customerID, netAmount).Scan(&attributionID, &programID, &referrerID, &referrerReward, &friendReward, &delayDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE referral_attributions SET status='reward_pending',qualified_at=now(),qualifying_transaction_id=$3,updated_at=now() WHERE company_id=$1 AND id=$2 AND status='registered'`, companyID, attributionID, transactionID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO customer_events(company_id,customer_id,event_type,transaction_id,source,properties,idempotency_key)
		VALUES($1,$2,'referral.converted',$3,'referral',jsonb_build_object('attributionId',$4::text,'referrerCustomerId',$5::text,'netAmount',$6::numeric),$7)
		ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, companyID, customerID, transactionID, attributionID, referrerID, netAmount, "referral-converted:"+attributionID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO customer_events(company_id,customer_id,event_type,transaction_id,source,properties,idempotency_key)
		VALUES($1,$2,'referral.converted',$3,'referral',jsonb_build_object('attributionId',$4::text,'referredCustomerId',$5::text,'netAmount',$6::numeric),$7)
		ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, companyID, referrerID, transactionID, attributionID, customerID, netAmount, "referral-converted-referrer:"+attributionID)
	if err != nil {
		return err
	}
	availableAt := fmt.Sprintf("now()+make_interval(days=>%d)", delayDays)
	_, err = tx.Exec(ctx, `INSERT INTO referral_rewards(company_id,attribution_id,beneficiary_customer_id,beneficiary_type,reward_type,reward_value,status,available_at,idempotency_key)
		SELECT $1,$2,$3,'referrer','points',$4,'pending',`+availableAt+`,$5 WHERE $4>0
		ON CONFLICT(company_id,idempotency_key) DO NOTHING`, companyID, attributionID, referrerID, referrerReward, "referral:"+attributionID+":referrer")
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO referral_rewards(company_id,attribution_id,beneficiary_customer_id,beneficiary_type,reward_type,reward_value,status,available_at,idempotency_key)
		SELECT $1,$2,$3,'friend','points',$4,'pending',`+availableAt+`,$5 WHERE $4>0
		ON CONFLICT(company_id,idempotency_key) DO NOTHING`, companyID, attributionID, customerID, friendReward, "referral:"+attributionID+":friend")
	return err
}

func applyCampaignAttribution(ctx context.Context, tx pgx.Tx, in *CanonicalTransaction, transactionID, customerID string) error {
	campaignID := in.CampaignID
	var recipientID string
	if campaignID == "" {
		err := tx.QueryRow(ctx, `SELECT c.id,r.id FROM campaign_recipients r JOIN marketing_campaigns c ON c.id=r.campaign_id AND c.company_id=r.company_id
			WHERE r.company_id=$1 AND r.customer_id=$2 AND r.experiment_group='treatment' AND r.status='sent' AND c.sent_at IS NOT NULL
			AND c.sent_at<=$3 AND $3<=c.sent_at+make_interval(days=>c.attribution_window_days)
			ORDER BY c.sent_at DESC LIMIT 1`, in.CompanyID, customerID, in.OccurredAt).Scan(&campaignID, &recipientID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		in.CampaignID = campaignID
		if _, err = tx.Exec(ctx, `UPDATE sales_transactions SET campaign_id=$3,updated_at=now() WHERE company_id=$1 AND id=$2`, in.CompanyID, transactionID, campaignID); err != nil {
			return err
		}
	} else {
		_ = tx.QueryRow(ctx, `SELECT id FROM campaign_recipients WHERE company_id=$1 AND campaign_id=$2 AND customer_id=$3`, in.CompanyID, campaignID, customerID).Scan(&recipientID)
	}
	_, err := tx.Exec(ctx, `INSERT INTO campaign_conversions(company_id,campaign_id,campaign_recipient_id,customer_id,transaction_id,conversion_type,conversion_value,currency,idempotency_key)
		VALUES($1,$2,nullif($3,'')::uuid,$4,$5,'purchased',$6,$7,$8) ON CONFLICT DO NOTHING`, in.CompanyID, campaignID, recipientID, customerID, transactionID, in.NetAmount, in.Currency, "campaign-purchase:"+transactionID)
	return err
}

func (s *Service) Refund(ctx context.Context, in RefundInput) (RefundResult, error) {
	if in.CompanyID == "" || in.OriginalID == "" || strings.TrimSpace(in.ExternalID) == "" || in.Amount <= 0 || strings.TrimSpace(in.Reason) == "" {
		return RefundResult{}, errors.New("original transaction, external id, positive amount and reason are required")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RefundResult{}, err
	}
	defer tx.Rollback(ctx)
	var provider, connectionID, currency, customerID, branchID string
	var netAmount float64
	var sandbox bool
	err = tx.QueryRow(ctx, `SELECT provider,coalesce(integration_connection_id::text,''),currency,coalesce(customer_id::text,''),coalesce(branch_id::text,''),net_amount,sandbox
		FROM sales_transactions WHERE company_id=$1 AND id=$2 AND status IN('completed','partially_refunded') FOR UPDATE`, in.CompanyID, in.OriginalID).Scan(&provider, &connectionID, &currency, &customerID, &branchID, &netAmount, &sandbox)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefundResult{}, errors.New("refundable transaction not found")
	}
	if err != nil {
		return RefundResult{}, err
	}
	if sandbox != in.Sandbox {
		return RefundResult{}, errors.New("transaction environment mismatch")
	}
	var alreadyRefunded float64
	err = tx.QueryRow(ctx, `SELECT coalesce(sum(net_amount),0) FROM sales_transactions WHERE company_id=$1 AND original_transaction_id=$2 AND status IN('refunded','partially_refunded')`, in.CompanyID, in.OriginalID).Scan(&alreadyRefunded)
	if err != nil {
		return RefundResult{}, err
	}
	remaining := netAmount - alreadyRefunded
	if in.Amount > remaining+0.0001 {
		return RefundResult{}, errors.New("refund amount exceeds remaining transaction amount")
	}
	var duplicateID string
	err = tx.QueryRow(ctx, `SELECT id FROM sales_transactions WHERE company_id=$1 AND provider=$2 AND external_id=$3`, in.CompanyID, provider, in.ExternalID).Scan(&duplicateID)
	if err == nil {
		return RefundResult{}, errors.New("refund external id already exists")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return RefundResult{}, err
	}
	newTotal := alreadyRefunded + in.Amount
	full := newTotal >= netAmount-0.0001
	refundStatus := "partially_refunded"
	originalStatus := "partially_refunded"
	if full {
		refundStatus = "refunded"
		originalStatus = "refunded"
	}
	var refundID string
	err = tx.QueryRow(ctx, `INSERT INTO sales_transactions(company_id,branch_id,customer_id,integration_connection_id,provider,external_id,status,occurred_at,gross_amount,net_amount,currency,source,original_transaction_id,idempotency_key,refund_reason,sandbox)
		VALUES($1,nullif($2,'')::uuid,nullif($3,'')::uuid,nullif($4,'')::uuid,$5,$6,$7,now(),$8,$8,$9,'refund',$10,$11,$12,$13) RETURNING id`, in.CompanyID, branchID, customerID, connectionID, provider, in.ExternalID, refundStatus, in.Amount, currency, in.OriginalID, "refund:"+provider+":"+in.ExternalID, strings.TrimSpace(in.Reason), sandbox).Scan(&refundID)
	if err != nil {
		return RefundResult{}, err
	}
	bonusRestored, bonusReversed := 0, 0
	if customerID != "" && !sandbox {
		var originalCredits, originalDebits int
		err = tx.QueryRow(ctx, `SELECT coalesce(sum(amount) FILTER(WHERE operation='credit'),0),coalesce(sum(amount) FILTER(WHERE operation='debit'),0) FROM bonus_ledger WHERE company_id=$1 AND sales_transaction_id=$2`, in.CompanyID, in.OriginalID).Scan(&originalCredits, &originalDebits)
		if err != nil {
			return RefundResult{}, err
		}
		ratio := in.Amount / netAmount
		bonusReversed = int(float64(originalCredits) * ratio)
		bonusRestored = int(float64(originalDebits) * ratio)
		if full {
			var reversedCredits, restoredDebits int
			_ = tx.QueryRow(ctx, `SELECT coalesce(sum(amount) FILTER(WHERE operation='debit'),0),coalesce(sum(amount) FILTER(WHERE operation='credit'),0) FROM bonus_ledger WHERE company_id=$1 AND sales_transaction_id IN(SELECT id FROM sales_transactions WHERE original_transaction_id=$2)`, in.CompanyID, in.OriginalID).Scan(&reversedCredits, &restoredDebits)
			bonusReversed = max(0, originalCredits-reversedCredits)
			bonusRestored = max(0, originalDebits-restoredDebits)
		}
		var balance int
		if err = tx.QueryRow(ctx, `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 FOR UPDATE`, in.CompanyID, customerID).Scan(&balance); err != nil {
			return RefundResult{}, err
		}
		if bonusReversed > balance {
			bonusReversed = balance
		}
		if bonusReversed > 0 {
			balance -= bonusReversed
			var reversalLedgerID string
			err = tx.QueryRow(ctx, `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description,idempotency_key,sales_transaction_id) VALUES($1,$2,'debit',$3,$4,'Сторно начисления при возврате',$5,$6) RETURNING id`, in.CompanyID, customerID, bonusReversed, balance, "refund:debit:"+provider+":"+in.ExternalID, refundID).Scan(&reversalLedgerID)
			if err == nil {
				err = ConsumeBonusLots(ctx, tx, in.CompanyID, customerID, reversalLedgerID, refundID, bonusReversed)
			}
		}
		if err == nil && bonusRestored > 0 {
			balance += bonusRestored
			var restoreLedgerID string
			err = tx.QueryRow(ctx, `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description,idempotency_key,sales_transaction_id) VALUES($1,$2,'credit',$3,$4,'Возврат списанных бонусов',$5,$6) RETURNING id`, in.CompanyID, customerID, bonusRestored, balance, "refund:credit:"+provider+":"+in.ExternalID, refundID).Scan(&restoreLedgerID)
			if err == nil {
				err = restoreBonusLots(ctx, tx, in.CompanyID, customerID, in.OriginalID, restoreLedgerID, refundID, bonusRestored)
			}
		}
		if err != nil {
			return RefundResult{}, err
		}
		_, err = tx.Exec(ctx, `UPDATE customers SET total_points=$3,updated_at=now() WHERE company_id=$1 AND id=$2`, in.CompanyID, customerID, balance)
		if err != nil {
			return RefundResult{}, err
		}
		if full {
			var affected bool
			_ = tx.QueryRow(ctx, `WITH changed AS (UPDATE visits SET reversed_at=now(),reversal_transaction_id=$3,reversed_points=points_added WHERE company_id=$1 AND sales_transaction_id=$2 AND reversed_at IS NULL RETURNING 1) SELECT EXISTS(SELECT 1 FROM changed)`, in.CompanyID, in.OriginalID, refundID).Scan(&affected)
			if affected {
				_, err = tx.Exec(ctx, `UPDATE customers SET total_visits=greatest(0,total_visits-1),updated_at=now() WHERE company_id=$1 AND id=$2`, in.CompanyID, customerID)
			}
		}
		if err != nil {
			return RefundResult{}, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE sales_transactions SET status=$3,updated_at=now() WHERE company_id=$1 AND id=$2`, in.CompanyID, in.OriginalID, originalStatus)
	if err != nil {
		return RefundResult{}, err
	}
	if !sandbox {
		if err = reverseReferralQualification(ctx, tx, in.CompanyID, in.OriginalID, refundID, max(0, remaining-in.Amount)); err != nil {
			return RefundResult{}, err
		}
	}
	if !sandbox {
		refundTransaction := CanonicalTransaction{CompanyID: in.CompanyID, Provider: provider, ExternalID: in.ExternalID, Status: refundStatus}
		if err = appendOutbox(ctx, tx, refundTransaction, refundID, customerID, branchID); err != nil {
			return RefundResult{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO integration_jobs(company_id,connection_id,job_type,resource,idempotency_key,payload)
			VALUES($1,$2,'analytics_projection','sales_transaction',$3,jsonb_build_object('transactionId',$4::text)) ON CONFLICT DO NOTHING`, in.CompanyID, connectionID, "analytics:"+refundID, refundID)
		if err != nil {
			return RefundResult{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return RefundResult{}, err
	}
	return RefundResult{TransactionID: refundID, OriginalID: in.OriginalID, RefundedAmount: in.Amount, RemainingAmount: max(0, remaining-in.Amount), OriginalStatus: originalStatus, BonusRestored: bonusRestored, BonusReversed: bonusReversed}, nil
}

func reverseReferralQualification(ctx context.Context, tx pgx.Tx, companyID, originalTransactionID, refundTransactionID string, remainingAmount float64) error {
	var attributionID string
	err := tx.QueryRow(ctx, `SELECT a.id FROM referral_attributions a JOIN referral_programs p ON p.id=a.program_id AND p.company_id=a.company_id
		WHERE a.company_id=$1 AND a.qualifying_transaction_id=$2 AND a.status IN('qualified','reward_pending','rewarded') AND $3<p.minimum_purchase_amount
		FOR UPDATE OF a`, companyID, originalTransactionID, remainingAmount).Scan(&attributionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id,beneficiary_customer_id,reward_value::integer,status FROM referral_rewards
		WHERE company_id=$1 AND attribution_id=$2 AND status IN('pending','issued') ORDER BY beneficiary_type FOR UPDATE`, companyID, attributionID)
	if err != nil {
		return err
	}
	type reward struct {
		id, customerID, status string
		amount                 int
	}
	rewards := []reward{}
	for rows.Next() {
		var item reward
		if err = rows.Scan(&item.id, &item.customerID, &item.amount, &item.status); err != nil {
			rows.Close()
			return err
		}
		rewards = append(rewards, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range rewards {
		unrecovered := 0
		if item.status == "issued" && item.amount > 0 {
			var balance int
			if err = tx.QueryRow(ctx, `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, companyID, item.customerID).Scan(&balance); err != nil {
				return err
			}
			debit := min(balance, item.amount)
			unrecovered = item.amount - debit
			if debit > 0 {
				var ledgerID string
				key := "referral-reversal:" + item.id
				err = tx.QueryRow(ctx, `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description,idempotency_key,sales_transaction_id)
					VALUES($1,$2,'debit',$3,$4,'Сторно реферальной награды после возврата',$5,$6)
					ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING RETURNING id`, companyID, item.customerID, debit, balance-debit, key, refundTransactionID).Scan(&ledgerID)
				if errors.Is(err, pgx.ErrNoRows) {
					err = nil
				} else if err == nil {
					err = ConsumeBonusLots(ctx, tx, companyID, item.customerID, ledgerID, refundTransactionID, debit)
					if err == nil {
						_, err = tx.Exec(ctx, `UPDATE customers SET total_points=total_points-$3,updated_at=now() WHERE company_id=$1 AND id=$2`, companyID, item.customerID, debit)
					}
				}
				if err != nil {
					return err
				}
			}
		}
		_, err = tx.Exec(ctx, `UPDATE referral_rewards SET status='reversed',reversed_at=now(),metadata=metadata||jsonb_build_object('refundTransactionId',$3::text,'unrecoveredPoints',$4::integer) WHERE company_id=$1 AND id=$2`, companyID, item.id, refundTransactionID, unrecovered)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE referral_attributions SET status='reversed',rejection_reason='qualifying_purchase_refunded',updated_at=now() WHERE company_id=$1 AND id=$2`, companyID, attributionID)
	return err
}

func restoreBonusLots(ctx context.Context, tx pgx.Tx, companyID, customerID, originalTransactionID, restoreLedgerID, refundTransactionID string, amount int) error {
	rows, err := tx.Query(ctx, `SELECT r.id,r.bonus_lot_id,r.redeemed_amount-r.restored_amount
		FROM bonus_lot_redemptions r JOIN bonus_ledger b ON b.id=r.debit_ledger_id
		WHERE r.company_id=$1 AND r.customer_id=$2 AND b.sales_transaction_id=$3 AND r.redeemed_amount>r.restored_amount
		ORDER BY r.created_at DESC,r.id FOR UPDATE`, companyID, customerID, originalTransactionID)
	if err != nil {
		return err
	}
	type redemption struct {
		id, lotID string
		available int
	}
	items := []redemption{}
	for rows.Next() {
		var item redemption
		if err = rows.Scan(&item.id, &item.lotID, &item.available); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	left := amount
	for _, item := range items {
		if left == 0 {
			break
		}
		restored := min(left, item.available)
		if _, err = tx.Exec(ctx, `UPDATE bonus_lot_redemptions SET restored_amount=restored_amount+$2,updated_at=now() WHERE id=$1`, item.id, restored); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE bonus_lots SET remaining_amount=remaining_amount+$2,status=CASE WHEN activates_at>now() THEN 'pending' ELSE 'active' END,updated_at=now() WHERE id=$1`, item.lotID, restored); err != nil {
			return err
		}
		left -= restored
	}
	if left > 0 {
		// Legacy transactions may predate allocation tracking; preserve the restored obligation as its own lot.
		_, err = tx.Exec(ctx, `INSERT INTO bonus_lots(company_id,customer_id,source_ledger_id,source_transaction_id,issued_amount,remaining_amount,monetary_value,issued_at,activates_at,status,metadata)
			VALUES($1,$2,$3,$4,$5::integer,$5::integer,$5::numeric,now(),now(),'active',jsonb_build_object('restoredFromTransaction',$6::text))`, companyID, customerID, restoreLedgerID, refundTransactionID, left, originalTransactionID)
	}
	return err
}

func resolveBranch(ctx context.Context, tx pgx.Tx, companyID, connectionID, directID, externalID string) (string, error) {
	if directID != "" {
		var id string
		err := tx.QueryRow(ctx, `SELECT id FROM branches WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL AND is_active`, companyID, directID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("branch does not belong to tenant")
		}
		return id, err
	}
	if externalID == "" {
		return "", nil
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT coalesce(branch_id::text,'') FROM integration_location_mappings WHERE company_id=$1 AND connection_id=$2 AND external_location_id=$3 AND status='mapped'`, companyID, connectionID, externalID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func resolveCustomer(ctx context.Context, tx pgx.Tx, companyID, connectionID, directID, externalID, phone string) (string, error) {
	if directID != "" {
		var id string
		err := tx.QueryRow(ctx, `SELECT id FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, companyID, directID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("customer does not belong to tenant")
		}
		return id, err
	}
	if externalID != "" {
		var id string
		err := tx.QueryRow(ctx, `SELECT coalesce(customer_id::text,'') FROM integration_customer_links WHERE company_id=$1 AND connection_id=$2 AND external_customer_id=$3 AND status='linked'`, companyID, connectionID, externalID).Scan(&id)
		if err == nil && id != "" {
			return id, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}
	normalized := NormalizePhone(phone)
	if normalized == "" {
		return "", nil
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM customers WHERE company_id=$1 AND regexp_replace(phone,'\D','','g')=trim(leading '+' from $2) AND deleted_at IS NULL ORDER BY created_at LIMIT 1`, companyID, normalized).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if externalID != "" {
		_, err = tx.Exec(ctx, `INSERT INTO integration_customer_links(company_id,connection_id,customer_id,external_customer_id,normalized_phone,status,match_method,last_synced_at)
			VALUES($1,$2,$3,$4,$5,'linked','phone',now()) ON CONFLICT(connection_id,external_customer_id) DO UPDATE SET customer_id=excluded.customer_id,normalized_phone=excluded.normalized_phone,status='linked',match_method='phone',last_synced_at=now(),updated_at=now()`, companyID, connectionID, id, externalID, normalized)
	}
	return id, err
}

func applyLoyalty(ctx context.Context, tx pgx.Tx, in CanonicalTransaction, transactionID, branchID, customerID string) (string, error) {
	if branchID == "" {
		return "", nil
	}
	var balance int
	err := tx.QueryRow(ctx, `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 FOR UPDATE`, in.CompanyID, customerID).Scan(&balance)
	if err != nil {
		return "", err
	}
	if in.BonusSpent > balance {
		return "", fmt.Errorf("bonus balance is insufficient")
	}
	var visitID string
	err = tx.QueryRow(ctx, `INSERT INTO visits(company_id,branch_id,customer_id,points_added,comment,idempotency_key,sales_transaction_id)
		VALUES($1,$2,$3,$4,'POS transaction',$5,$6) RETURNING id`, in.CompanyID, branchID, customerID, in.BonusEarned, "visit:"+in.Provider+":"+in.ExternalID, transactionID).Scan(&visitID)
	if err != nil {
		return "", err
	}
	if in.BonusSpent > 0 {
		balance -= in.BonusSpent
		var debitLedgerID string
		err = tx.QueryRow(ctx, `INSERT INTO bonus_ledger(company_id,customer_id,visit_id,operation,amount,balance_after,description,idempotency_key,sales_transaction_id)
			VALUES($1,$2,$3,'debit',$4,$5,'Списание по POS-чеку',$6,$7) RETURNING id`, in.CompanyID, customerID, visitID, in.BonusSpent, balance, "bonus:debit:"+in.Provider+":"+in.ExternalID, transactionID).Scan(&debitLedgerID)
		if err != nil {
			return "", err
		}
		if err = ConsumeBonusLots(ctx, tx, in.CompanyID, customerID, debitLedgerID, transactionID, in.BonusSpent); err != nil {
			return "", err
		}
		if err = appendLoyaltyEvent(ctx, tx, in, customerID, transactionID, branchID, "bonus.spent", "pos-bonus-spent:"+in.Provider+":"+in.ExternalID, map[string]any{"amount": in.BonusSpent, "balanceAfter": balance, "reason": "Списание по POS-чеку"}); err != nil {
			return "", err
		}
	}
	if in.BonusEarned > 0 {
		balance += in.BonusEarned
		var creditLedgerID string
		err = tx.QueryRow(ctx, `INSERT INTO bonus_ledger(company_id,customer_id,visit_id,operation,amount,balance_after,description,idempotency_key,sales_transaction_id)
			VALUES($1,$2,$3,'credit',$4,$5,'Начисление по POS-чеку',$6,$7) RETURNING id`, in.CompanyID, customerID, visitID, in.BonusEarned, balance, "bonus:credit:"+in.Provider+":"+in.ExternalID, transactionID).Scan(&creditLedgerID)
		if err != nil {
			return "", err
		}
		if err = IssueBonusLot(ctx, tx, in.CompanyID, customerID, creditLedgerID, transactionID, in.BonusEarned); err != nil {
			return "", err
		}
		if err = appendLoyaltyEvent(ctx, tx, in, customerID, transactionID, branchID, "bonus.earned", "pos-bonus-earned:"+in.Provider+":"+in.ExternalID, map[string]any{"amount": in.BonusEarned, "balanceAfter": balance, "reason": "Начисление по POS-чеку"}); err != nil {
			return "", err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE customers SET total_points=$3,total_visits=total_visits+1,updated_at=now() WHERE company_id=$1 AND id=$2`, in.CompanyID, customerID, balance)
	if err == nil {
		err = appendLoyaltyEvent(ctx, tx, in, customerID, transactionID, branchID, "visit.completed", "pos-visit:"+in.Provider+":"+in.ExternalID, map[string]any{"visitId": visitID, "pointsAdded": in.BonusEarned, "source": "pos"})
	}
	return visitID, err
}

func appendLoyaltyEvent(ctx context.Context, tx pgx.Tx, in CanonicalTransaction, customerID, transactionID, branchID, eventType, idempotencyKey string, properties map[string]any) error {
	payload, err := json.Marshal(properties)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO customer_events(company_id,customer_id,event_type,occurred_at,branch_id,transaction_id,source,properties,idempotency_key)
		VALUES($1,$2,$3,$4,nullif($5,'')::uuid,$6,$7,$8::jsonb,$9)
		ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, in.CompanyID, customerID, eventType, in.OccurredAt, branchID, transactionID, in.Source, payload, idempotencyKey)
	return err
}

func IssueBonusLot(ctx context.Context, tx pgx.Tx, companyID, customerID, creditLedgerID, transactionID string, amount int) error {
	_, err := tx.Exec(ctx, `INSERT INTO bonus_lots(company_id,customer_id,source_ledger_id,source_transaction_id,issued_amount,remaining_amount,monetary_value,issued_at,activates_at,status)
		VALUES($1,$2,$3,nullif($4,'')::uuid,$5::integer,$5::integer,$5::numeric,now(),now(),'active')`, companyID, customerID, creditLedgerID, transactionID, amount)
	return err
}

func ConsumeBonusLots(ctx context.Context, tx pgx.Tx, companyID, customerID, debitLedgerID, transactionID string, amount int) error {
	rows, err := tx.Query(ctx, `SELECT id,remaining_amount FROM bonus_lots
		WHERE company_id=$1 AND customer_id=$2 AND status IN('pending','active') AND activates_at<=now()
		AND (expires_at IS NULL OR expires_at>now()) AND remaining_amount>0
		ORDER BY activates_at,expires_at NULLS LAST,issued_at,id FOR UPDATE`, companyID, customerID)
	if err != nil {
		return err
	}
	type lot struct {
		id        string
		remaining int
	}
	lots := []lot{}
	for rows.Next() {
		var item lot
		if err = rows.Scan(&item.id, &item.remaining); err != nil {
			rows.Close()
			return err
		}
		lots = append(lots, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	left := amount
	for _, item := range lots {
		if left == 0 {
			break
		}
		used := min(left, item.remaining)
		if _, err = tx.Exec(ctx, `UPDATE bonus_lots SET remaining_amount=remaining_amount-$2,status=CASE WHEN remaining_amount-$2=0 THEN 'redeemed' ELSE 'active' END,updated_at=now() WHERE id=$1`, item.id, used); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO bonus_lot_redemptions(company_id,customer_id,bonus_lot_id,debit_ledger_id,transaction_id,redeemed_amount) VALUES($1,$2,$3,$4,nullif($5,'')::uuid,$6)`, companyID, customerID, item.id, debitLedgerID, transactionID, used); err != nil {
			return err
		}
		left -= used
	}
	if left > 0 {
		return fmt.Errorf("active bonus lots are insufficient")
	}
	return nil
}

func appendPurchaseEvent(ctx context.Context, tx pgx.Tx, companyID, customerID, transactionID, branchID string, in CanonicalTransaction) error {
	var purchaseCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM sales_transactions WHERE company_id=$1 AND customer_id=$2 AND status='completed'`, companyID, customerID).Scan(&purchaseCount); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO customer_events(company_id,customer_id,event_type,occurred_at,branch_id,transaction_id,campaign_id,source,properties,idempotency_key)
		VALUES($1,$2,'purchase.completed',$3,nullif($4,'')::uuid,$5,nullif($6,'')::uuid,$7,jsonb_build_object('netAmount',$8::numeric,'currency',$9::text,'purchaseNumber',$10::integer),$11)`, companyID, customerID, in.OccurredAt, branchID, transactionID, in.CampaignID, in.Source, in.NetAmount, in.Currency, purchaseCount, "event:"+in.Provider+":"+in.ExternalID)
	if err == nil && purchaseCount == 2 {
		_, err = tx.Exec(ctx, `INSERT INTO customer_events(company_id,customer_id,event_type,occurred_at,branch_id,transaction_id,source,properties,idempotency_key)
			VALUES($1,$2,'customer.returned',$3,nullif($4,'')::uuid,$5,$6,jsonb_build_object('reason','second_purchase','purchaseNumber',2),$7)
			ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, companyID, customerID, in.OccurredAt, branchID, transactionID, in.Source, "return:"+in.Provider+":"+in.ExternalID)
	}
	return err
}

func appendOutbox(ctx context.Context, tx pgx.Tx, in CanonicalTransaction, transactionID, customerID, branchID string) error {
	eventType := "SalesTransactionCompleted"
	if in.Status == "refunded" || in.Status == "partially_refunded" {
		eventType = "SalesTransactionRefunded"
	} else if in.Status == "cancelled" {
		eventType = "SalesTransactionCancelled"
	}
	_, err := tx.Exec(ctx, `INSERT INTO outbox_events(company_id,event_type,aggregate_type,aggregate_id,payload,idempotency_key)
		VALUES($1,$2,'sales_transaction',$3::uuid,jsonb_build_object('transactionId',$3::text,'customerId',$4::text,'branchId',$5::text,'provider',$6::text,'externalId',$7::text),$8)`, in.CompanyID, eventType, transactionID, customerID, branchID, in.Provider, in.ExternalID, "outbox:"+in.Provider+":"+in.ExternalID)
	return err
}
