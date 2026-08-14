package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// appendCustomerEvent is the canonical write path for customer-facing product
// events. Idempotency makes webhook and operation retries safe.
func appendCustomerEvent(r *http.Request, tx pgx.Tx, companyID, customerID, eventType, branchID, idempotencyKey string, properties map[string]any) error {
	payload, err := json.Marshal(properties)
	if err != nil {
		return err
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO customer_events(company_id,customer_id,event_type,branch_id,source,properties,idempotency_key)
		VALUES($1,$2,$3,nullif($4,'')::uuid,'tappix',$5::jsonb,nullif($6,''))
		ON CONFLICT(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, companyID, customerID, eventType, branchID, payload, idempotencyKey)
	return err
}

func (a *api) customerTimeline(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT e.id,e.event_type,e.occurred_at,coalesce(e.source,''),e.properties,coalesce(b.name,'')
		FROM customer_events e LEFT JOIN branches b ON b.id=e.branch_id AND b.company_id=e.company_id
		WHERE e.company_id=$1 AND e.customer_id=$2 ORDER BY e.occurred_at DESC,e.created_at DESC LIMIT 200`, companyID(r), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить события клиента")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, eventType, source, branch string
		var occurred time.Time
		var properties map[string]any
		if rows.Scan(&id, &eventType, &occurred, &source, &properties, &branch) == nil {
			items = append(items, map[string]any{"id": id, "type": eventType, "occurredAt": occurred, "source": source, "branch": branch, "properties": properties})
		}
	}
	write(w, 200, envelope{Success: true, Data: items})
}
