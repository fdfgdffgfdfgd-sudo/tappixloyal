package httpapi

import "net/http"

func (a *api) publicPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT code,name,monthly_price,coalesce(annual_price,monthly_price*10),currency,trial_days,description,highlighted FROM plans_v2 WHERE status='active' ORDER BY monthly_price`)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить тарифы")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var code, name, currency, description string
		var monthly, annual float64
		var trialDays int
		var highlighted bool
		if err := rows.Scan(&code, &name, &monthly, &annual, &currency, &trialDays, &description, &highlighted); err != nil {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить тарифы")
			return
		}
		items = append(items, map[string]any{"id": code, "name": name, "monthlyPrice": monthly, "annualPrice": annual, "currency": currency, "trialDays": trialDays, "description": description, "highlighted": highlighted})
	}
	if rows.Err() != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить тарифы")
		return
	}
	write(w, 200, envelope{Success: true, Data: items})
}
