package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

type partnershipInput struct {
	PartnerCompanyID string `json:"partnerCompanyId"`
	Name             string `json:"name"`
}
type partnershipOfferInput struct {
	SourceCompanyID       string  `json:"sourceCompanyId"`
	RewardCompanyID       string  `json:"rewardCompanyId"`
	Code                  string  `json:"code"`
	Name                  string  `json:"name"`
	RewardPoints          int     `json:"rewardPoints"`
	MinimumSourcePurchase float64 `json:"minimumSourcePurchase"`
	MaxRedemptions        *int    `json:"maxRedemptions"`
	MaxPerCustomer        int     `json:"maxPerCustomer"`
	EndsAt                string  `json:"endsAt"`
}
type partnershipRedeemInput struct {
	Code                string `json:"code"`
	TargetCustomerID    string `json:"targetCustomerId"`
	SourceTransactionID string `json:"sourceTransactionId"`
	IdempotencyKey      string `json:"idempotencyKey"`
}

func (a *api) listPartnerships(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT p.id,p.name,p.status,p.initiator_company_id,p.partner_company_id,ci.name,cp.name,p.approved_at,p.created_at,
		(SELECT count(*) FROM partnership_offers o WHERE o.partnership_id=p.id),(SELECT count(*) FROM partnership_redemptions x WHERE x.partnership_id=p.id AND x.status='rewarded')
		FROM business_partnerships p JOIN companies ci ON ci.id=p.initiator_company_id JOIN companies cp ON cp.id=p.partner_company_id
		WHERE p.initiator_company_id=$1 OR p.partner_company_id=$1 ORDER BY p.created_at DESC`, companyID(r))
	if err != nil {
		fail(w, 500, "PARTNERSHIPS_FAILED", "Не удалось загрузить партнёрства")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, status, initiator, partner, initiatorName, partnerName string
		var approved any
		var created any
		var offers, redemptions int
		if rows.Scan(&id, &name, &status, &initiator, &partner, &initiatorName, &partnerName, &approved, &created, &offers, &redemptions) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "status": status, "initiatorCompanyId": initiator, "partnerCompanyId": partner, "initiatorName": initiatorName, "partnerName": partnerName, "approvedAt": approved, "createdAt": created, "offers": offers, "redemptions": redemptions})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}

func (a *api) createPartnership(w http.ResponseWriter, r *http.Request) {
	var in partnershipInput
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.PartnerCompanyID == "" || in.PartnerCompanyID == companyID(r) || in.Name == "" {
		fail(w, 422, "INVALID_PARTNERSHIP", "Выберите другой бизнес и укажите название")
		return
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO business_partnerships(initiator_company_id,partner_company_id,name,created_by) SELECT $1,c.id,$3,$4 FROM companies c WHERE c.id=$2 AND c.status='active' AND c.deleted_at IS NULL RETURNING id`, companyID(r), in.PartnerCompanyID, in.Name, claims.Subject).Scan(&id)
	if err != nil {
		fail(w, 409, "PARTNERSHIP_CREATE_FAILED", "Партнёр недоступен или приглашение уже существует")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "status": "pending"}})
}

func (a *api) approvePartnership(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	tag, err := a.db.Exec(r.Context(), `UPDATE business_partnerships SET status='active',approved_by=$3,approved_at=now(),updated_at=now() WHERE id=$1 AND partner_company_id=$2 AND status='pending'`, r.PathValue("id"), companyID(r), claims.Subject)
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "PARTNERSHIP_NOT_PENDING", "Ожидающее приглашение не найдено")
		return
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"status": "active"}})
}

func (a *api) createPartnershipOffer(w http.ResponseWriter, r *http.Request) {
	var in partnershipOfferInput
	if !decode(w, r, &in) {
		return
	}
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" {
		value, err := randomHex(5)
		if err != nil {
			fail(w, 500, "CODE_FAILED", "Не удалось создать код")
			return
		}
		in.Code = strings.ToUpper(value)
	}
	if in.MaxPerCustomer == 0 {
		in.MaxPerCustomer = 1
	}
	if in.Name == "" || in.RewardPoints <= 0 || in.MinimumSourcePurchase < 0 || in.MaxPerCustomer < 1 || in.MaxPerCustomer > 100 || in.SourceCompanyID == in.RewardCompanyID {
		fail(w, 422, "INVALID_OFFER", "Проверьте компании, награду и лимиты")
		return
	}
	claims, _ := r.Context().Value(identityKey).(tokenClaims)
	var id string
	var ends any
	if in.EndsAt != "" {
		ends = in.EndsAt
	}
	err := a.db.QueryRow(r.Context(), `INSERT INTO partnership_offers(partnership_id,source_company_id,reward_company_id,code,name,reward_points,minimum_source_purchase,max_redemptions,max_per_customer,ends_at,created_by)
		SELECT p.id,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12 FROM business_partnerships p WHERE p.id=$1 AND p.status='active' AND ($2=p.initiator_company_id OR $2=p.partner_company_id)
		AND (($3=p.initiator_company_id AND $4=p.partner_company_id) OR ($4=p.initiator_company_id AND $3=p.partner_company_id)) RETURNING id`, r.PathValue("id"), companyID(r), in.SourceCompanyID, in.RewardCompanyID, in.Code, in.Name, in.RewardPoints, in.MinimumSourcePurchase, in.MaxRedemptions, in.MaxPerCustomer, ends, claims.Subject).Scan(&id)
	if err != nil {
		fail(w, 409, "OFFER_CREATE_FAILED", "Не удалось создать предложение; проверьте партнёрство и уникальность кода")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"id": id, "code": in.Code}})
}

func (a *api) redeemPartnershipOffer(w http.ResponseWriter, r *http.Request) {
	var in partnershipRedeemInput
	if !decode(w, r, &in) {
		return
	}
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.Code == "" || in.TargetCustomerID == "" || in.SourceTransactionID == "" || in.IdempotencyKey == "" {
		fail(w, 422, "INVALID_REDEMPTION", "Укажите код, чек, клиента-получателя и idempotencyKey")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось начать операцию")
		return
	}
	defer tx.Rollback(r.Context())
	var offerID, partnershipID, rewardCompanyID string
	var points, maxPer int
	var minPurchase float64
	var maxRedemptions *int
	err = tx.QueryRow(r.Context(), `SELECT o.id,o.partnership_id,o.reward_company_id,o.reward_points,o.minimum_source_purchase,o.max_redemptions,o.max_per_customer FROM partnership_offers o JOIN business_partnerships p ON p.id=o.partnership_id
		WHERE lower(o.code)=lower($1) AND o.source_company_id=$2 AND o.is_active AND p.status='active' AND o.starts_at<=now() AND (o.ends_at IS NULL OR o.ends_at>now()) FOR UPDATE`, in.Code, companyID(r)).Scan(&offerID, &partnershipID, &rewardCompanyID, &points, &minPurchase, &maxRedemptions, &maxPer)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "OFFER_NOT_FOUND", "Промокод не найден или неактивен")
		return
	}
	if err != nil {
		fail(w, 500, "REDEMPTION_FAILED", "Не удалось проверить промокод")
		return
	}
	var purchase float64
	err = tx.QueryRow(r.Context(), `SELECT net_amount FROM sales_transactions WHERE company_id=$1 AND id=$2 AND original_transaction_id IS NULL AND status IN('completed','partially_refunded') AND NOT sandbox`, companyID(r), in.SourceTransactionID).Scan(&purchase)
	if err != nil || purchase < minPurchase {
		fail(w, 409, "PURCHASE_NOT_QUALIFIED", "Чек не найден или его сумма ниже минимальной")
		return
	}
	var balance int
	err = tx.QueryRow(r.Context(), `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, rewardCompanyID, in.TargetCustomerID).Scan(&balance)
	if err != nil {
		fail(w, 409, "TARGET_CUSTOMER_REQUIRED", "Клиент должен быть зарегистрирован у бизнеса, выдающего бонус")
		return
	}
	var totalCount, customerCount int
	_ = tx.QueryRow(r.Context(), `SELECT count(*),count(*) FILTER(WHERE target_customer_id=$2) FROM partnership_redemptions WHERE offer_id=$1 AND status='rewarded'`, offerID, in.TargetCustomerID).Scan(&totalCount, &customerCount)
	if (maxRedemptions != nil && totalCount >= *maxRedemptions) || customerCount >= maxPer {
		fail(w, 409, "REDEMPTION_LIMIT_REACHED", "Лимит использования промокода исчерпан")
		return
	}
	var ledgerID string
	err = tx.QueryRow(r.Context(), `INSERT INTO bonus_ledger(company_id,customer_id,operation,amount,balance_after,description,idempotency_key) VALUES($1,$2,'credit',$3,$4,'Бонус от партнёра',$5) RETURNING id`, rewardCompanyID, in.TargetCustomerID, points, balance+points, "partner:"+offerID+":"+in.IdempotencyKey).Scan(&ledgerID)
	if err == nil {
		err = posintegration.IssueBonusLot(r.Context(), tx, rewardCompanyID, in.TargetCustomerID, ledgerID, "", points)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE customers SET total_points=$3,updated_at=now() WHERE company_id=$1 AND id=$2`, rewardCompanyID, in.TargetCustomerID, balance+points)
	}
	var redemptionID string
	if err == nil {
		err = tx.QueryRow(r.Context(), `INSERT INTO partnership_redemptions(partnership_id,offer_id,source_company_id,reward_company_id,target_customer_id,source_transaction_id,bonus_ledger_id,reward_points,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, partnershipID, offerID, companyID(r), rewardCompanyID, in.TargetCustomerID, in.SourceTransactionID, ledgerID, points, in.IdempotencyKey).Scan(&redemptionID)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 409, "REDEMPTION_FAILED", "Промокод уже применён или операция не выполнена")
		return
	}
	write(w, 201, envelope{Success: true, Data: map[string]any{"redemptionId": redemptionID, "rewardCompanyId": rewardCompanyID, "points": points, "balance": balance + points}})
}
