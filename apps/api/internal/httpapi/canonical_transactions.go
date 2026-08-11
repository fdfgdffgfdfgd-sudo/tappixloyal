package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	posintegration "github.com/tappix/platform/apps/api/internal/integration"
)

type apiKeyContextKey string

const (
	apiKeyScopesKey  apiKeyContextKey = "api-key-scopes"
	apiKeySandboxKey apiKeyContextKey = "api-key-sandbox"
)

func (a *api) authenticateAPIKey(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := strings.TrimSpace(r.Header.Get("X-Tappix-API-Key"))
		if secret == "" {
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "ApiKey ") {
				secret = strings.TrimSpace(strings.TrimPrefix(header, "ApiKey "))
			}
		}
		if !strings.HasPrefix(secret, "tpx_") {
			fail(w, 401, "API_KEY_REQUIRED", "Передайте API-ключ")
			return
		}
		sum := sha256.Sum256([]byte(secret))
		hash := hex.EncodeToString(sum[:])
		var keyID, tenant string
		var scopes []string
		var sandbox bool
		err := a.db.QueryRow(r.Context(), `UPDATE api_keys SET last_used_at=now() WHERE secret_hash=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>now()) AND $2=ANY(scopes) RETURNING id,company_id,scopes,sandbox`, hash, scope).Scan(&keyID, &tenant, &scopes, &sandbox)
		if err != nil {
			fail(w, 403, "API_KEY_SCOPE_DENIED", "API-ключ недействителен или не имеет требуемого scope")
			return
		}
		var active bool
		if a.db.QueryRow(r.Context(), `SELECT status='active' FROM companies WHERE id=$1 AND deleted_at IS NULL`, tenant).Scan(&active) != nil || !active {
			fail(w, 403, "COMPANY_BLOCKED", "Доступ компании приостановлен")
			return
		}
		claims := tokenClaims{Subject: keyID, CompanyID: tenant, Role: "api_key", ExpiresAt: time.Now().Add(time.Minute).Unix()}
		ctx := context.WithValue(r.Context(), identityKey, claims)
		ctx = context.WithValue(ctx, apiKeyScopesKey, scopes)
		ctx = context.WithValue(ctx, apiKeySandboxKey, sandbox)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *api) canonicalTransactionQuote(w http.ResponseWriter, r *http.Request) {
	var in posintegration.CanonicalTransaction
	if !decode(w, r, &in) {
		return
	}
	tenant := companyID(r)
	sandbox, _ := r.Context().Value(apiKeySandboxKey).(bool)
	if err := a.validateCanonicalOwnership(r, tenant, in); err != nil {
		fail(w, 422, "TENANT_REFERENCE_INVALID", err.Error())
		return
	}
	points := a.pointsForEvent(r.Context(), tenant, "visit_created", 20)
	balance := 0
	if in.CustomerID != "" {
		_ = a.db.QueryRow(r.Context(), `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, tenant, in.CustomerID).Scan(&balance)
	}
	maxSpend := balance
	if float64(maxSpend) > in.NetAmount {
		maxSpend = int(in.NetAmount)
	}
	requested := in.BonusSpent
	if requested > maxSpend {
		requested = maxSpend
	}
	if requested < 0 {
		requested = 0
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{
		"sandbox": sandbox, "currency": defaultCurrency(in.Currency), "customerId": in.CustomerID,
		"balance": balance, "maximumBonusSpend": maxSpend, "bonusSpent": requested,
		"bonusEarned": points, "payableAmount": maxFloat(0, in.NetAmount-float64(requested)),
		"mutated": false, "expiresAt": time.Now().Add(5 * time.Minute),
	}})
}

func (a *api) canonicalTransactionCreate(w http.ResponseWriter, r *http.Request) {
	var in posintegration.CanonicalTransaction
	if !decode(w, r, &in) {
		return
	}
	tenant := companyID(r)
	if err := a.validateCanonicalOwnership(r, tenant, in); err != nil {
		fail(w, 422, "TENANT_REFERENCE_INVALID", err.Error())
		return
	}
	in.CompanyID = tenant
	in.Sandbox, _ = r.Context().Value(apiKeySandboxKey).(bool)
	if in.Status == "" {
		in.Status = "completed"
	}
	if in.Status != "completed" {
		fail(w, 422, "TRANSACTION_STATUS_INVALID", "Endpoint закрытия принимает только completed-чек")
		return
	}
	in.BonusEarned = a.pointsForEvent(r.Context(), tenant, "visit_created", 20)
	if in.CustomerID != "" && in.BonusSpent > 0 {
		var balance int
		if err := a.db.QueryRow(r.Context(), `SELECT total_points FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, tenant, in.CustomerID).Scan(&balance); err != nil || in.BonusSpent > balance || float64(in.BonusSpent) > in.NetAmount {
			fail(w, 409, "BONUS_SPEND_INVALID", "Запрошенное списание превышает баланс или сумму чека")
			return
		}
	}
	result, err := a.integrationService.Ingest(r.Context(), in)
	if err != nil {
		fail(w, 422, "TRANSACTION_REJECTED", err.Error())
		return
	}
	status := 201
	if result.Duplicate {
		status = 200
	}
	write(w, status, envelope{Success: true, Data: result})
}

func (a *api) canonicalTransactionGet(w http.ResponseWriter, r *http.Request) {
	tenant := companyID(r)
	sandbox, _ := r.Context().Value(apiKeySandboxKey).(bool)
	var data map[string]any
	err := a.db.QueryRow(r.Context(), `SELECT jsonb_build_object(
		'id',t.id,'provider',t.provider,'externalId',t.external_id,'status',t.status,'occurredAt',t.occurred_at,
		'grossAmount',t.gross_amount,'discountAmount',t.discount_amount,'bonusPaidAmount',t.bonus_paid_amount,
		'cashPaidAmount',t.cash_paid_amount,'netAmount',t.net_amount,'costAmount',t.cost_amount,'currency',t.currency,
		'receiptNumber',t.receipt_number,'source',t.source,'customerId',t.customer_id,'branchId',t.branch_id,
		'connectionId',t.integration_connection_id,'campaignId',t.campaign_id,'originalTransactionId',t.original_transaction_id,
		'refundReason',t.refund_reason,'sandbox',t.sandbox,'createdAt',t.created_at,'updatedAt',t.updated_at,
		'items',coalesce((SELECT jsonb_agg(jsonb_build_object('id',i.id,'externalId',i.external_item_id,'name',i.name,'quantity',i.quantity,'unitPrice',i.unit_price,'netAmount',i.net_amount)) FROM sales_transaction_items i WHERE i.company_id=t.company_id AND i.transaction_id=t.id),'[]'::jsonb),
		'payments',coalesce((SELECT jsonb_agg(jsonb_build_object('id',p.id,'externalId',p.external_id,'type',p.payment_type,'status',p.status,'amount',p.amount)) FROM payments p WHERE p.company_id=t.company_id AND p.transaction_id=t.id),'[]'::jsonb)
	) FROM sales_transactions t WHERE t.company_id=$1 AND t.id=$2 AND t.sandbox=$3`, tenant, r.PathValue("id"), sandbox).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "TRANSACTION_NOT_FOUND", "Чек не найден")
		return
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить чек")
		return
	}
	write(w, 200, envelope{Success: true, Data: data})
}

func (a *api) canonicalTransactionRefund(w http.ResponseWriter, r *http.Request) {
	var in posintegration.RefundInput
	if !decode(w, r, &in) {
		return
	}
	in.CompanyID = companyID(r)
	in.OriginalID = r.PathValue("id")
	in.Sandbox, _ = r.Context().Value(apiKeySandboxKey).(bool)
	result, err := a.integrationService.Refund(r.Context(), in)
	if err != nil {
		fail(w, 422, "REFUND_REJECTED", err.Error())
		return
	}
	write(w, 201, envelope{Success: true, Data: result})
}

func (a *api) integrationJobRetry(w http.ResponseWriter, r *http.Request) {
	tenant := companyID(r)
	tag, err := a.db.Exec(r.Context(), `UPDATE integration_jobs SET status='pending',attempts=0,available_at=now(),last_error=NULL,retried_at=now()
		WHERE company_id=$1 AND id=$2 AND status IN('failed','dead')`, tenant, r.PathValue("id"))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось повторить задачу")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "JOB_NOT_RETRYABLE", "Задача не найдена или не требует повтора")
		return
	}
	write(w, 202, envelope{Success: true, Data: map[string]any{"id": r.PathValue("id"), "status": "pending"}})
}

func (a *api) validateCanonicalOwnership(r *http.Request, tenant string, in posintegration.CanonicalTransaction) error {
	if in.ConnectionID == "" {
		return errors.New("connectionId обязателен")
	}
	var connection bool
	if err := a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM integration_connections WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL AND status<>'disabled')`, tenant, in.ConnectionID).Scan(&connection); err != nil || !connection {
		return errors.New("подключение не принадлежит компании")
	}
	if in.BranchID != "" {
		var exists bool
		_ = a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM branches WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL AND is_active)`, tenant, in.BranchID).Scan(&exists)
		if !exists {
			return errors.New("филиал не принадлежит компании")
		}
	}
	if in.CustomerID != "" {
		var exists bool
		_ = a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM customers WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL)`, tenant, in.CustomerID).Scan(&exists)
		if !exists {
			return errors.New("клиент не принадлежит компании")
		}
	}
	return nil
}

func defaultCurrency(value string) string {
	if strings.TrimSpace(value) == "" {
		return "KZT"
	}
	return strings.ToUpper(value)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
