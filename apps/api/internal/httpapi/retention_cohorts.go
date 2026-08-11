package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type cohortCell struct {
	Period   int     `json:"period"`
	Label    string  `json:"label"`
	Retained int     `json:"retainedCustomers"`
	Rate     float64 `json:"retentionRate"`
}

func (a *api) retentionCohorts(w http.ResponseWriter, r *http.Request) {
	cohortType := r.URL.Query().Get("cohortType")
	if cohortType == "" {
		cohortType = "registration"
	}
	if cohortType != "registration" && cohortType != "first_purchase" {
		fail(w, 422, "INVALID_COHORT_TYPE", "cohortType должен быть registration или first_purchase")
		return
	}
	periods := 4
	if value := r.URL.Query().Get("periods"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 24 {
			fail(w, 422, "INVALID_PERIODS", "periods должен быть от 1 до 24")
			return
		}
		periods = parsed
	}
	grain, prefix := "month", "M"
	if cohortType == "first_purchase" {
		grain, prefix = "week", "W"
	}
	branchID := r.URL.Query().Get("branchId")
	source := r.URL.Query().Get("source")
	campaignID := r.URL.Query().Get("campaignId")
	programID := r.URL.Query().Get("programId")

	baseExpression := "date_trunc('month',c.created_at)"
	if cohortType == "first_purchase" {
		baseExpression = "date_trunc('week',min(p.occurred_at))"
	}
	baseJoin, baseGroup := "", ""
	if cohortType == "first_purchase" {
		baseJoin = `JOIN sales_transactions p ON p.company_id=c.company_id AND p.customer_id=c.id AND p.original_transaction_id IS NULL AND p.status IN('completed','partially_refunded','refunded') AND NOT p.sandbox`
		baseGroup = "GROUP BY c.id,c.company_id,c.created_at,fa.source,fa.campaign_id"
	}
	periodExpression := `extract(year FROM age(date_trunc('month',a.occurred_at),b.cohort_start))*12+extract(month FROM age(date_trunc('month',a.occurred_at),b.cohort_start))`
	if grain == "week" {
		periodExpression = `floor(extract(epoch FROM(date_trunc('week',a.occurred_at)-b.cohort_start))/604800)`
	}
	query := fmt.Sprintf(`WITH first_attribution AS (
		SELECT DISTINCT ON(customer_id) customer_id,source,campaign_id FROM customer_attributions WHERE company_id=$1 ORDER BY customer_id,attributed_at
	), base AS (
		SELECT c.id customer_id,%s cohort_start,coalesce(fa.source,'unknown') acquisition_source,fa.campaign_id
		FROM customers c %s LEFT JOIN first_attribution fa ON fa.customer_id=c.id
		WHERE c.company_id=$1 AND c.deleted_at IS NULL AND ($3='' OR coalesce(fa.source,'unknown')=$3) AND ($4='' OR fa.campaign_id::text=$4)
			AND ($2='' OR EXISTS(SELECT 1 FROM sales_transactions sf WHERE sf.company_id=$1 AND sf.customer_id=c.id AND sf.branch_id::text=$2 AND sf.original_transaction_id IS NULL AND NOT sf.sandbox))
			AND ($5='' OR EXISTS(SELECT 1 FROM sales_transactions sp WHERE sp.company_id=$1 AND sp.customer_id=c.id AND coalesce(sp.metadata->>'programId','')=$5 AND sp.original_transaction_id IS NULL AND NOT sp.sandbox)) %s
	), sizes AS (
		SELECT cohort_start,count(DISTINCT customer_id) cohort_size FROM base GROUP BY cohort_start
	), activity AS (
		SELECT b.cohort_start,b.customer_id,(%s)::integer period_index
		FROM base b JOIN sales_transactions a ON a.company_id=$1 AND a.customer_id=b.customer_id
			AND a.original_transaction_id IS NULL AND a.status IN('completed','partially_refunded','refunded') AND NOT a.sandbox
		WHERE a.occurred_at>=b.cohort_start AND ($2='' OR a.branch_id::text=$2)
			AND ($5='' OR coalesce(a.metadata->>'programId','')=$5)
	), retained AS (
		SELECT cohort_start,period_index,count(DISTINCT customer_id) retained FROM activity WHERE period_index BETWEEN 0 AND $6-1 GROUP BY cohort_start,period_index
	) SELECT s.cohort_start,s.cohort_size,r.period_index,r.retained
	FROM sizes s LEFT JOIN retained r USING(cohort_start) ORDER BY s.cohort_start DESC,r.period_index`, baseExpression, baseJoin, baseGroup, periodExpression)
	rows, err := a.db.Query(r.Context(), query, companyID(r), branchID, source, campaignID, programID, periods)
	if err != nil {
		fail(w, 500, "RETENTION_FAILED", "Не удалось построить когорты удержания")
		return
	}
	defer rows.Close()
	type cohort struct {
		Start time.Time
		Size  int
		Cells []cohortCell
	}
	order := []string{}
	cohorts := map[string]*cohort{}
	for rows.Next() {
		var start time.Time
		var size int
		var period, retained *int
		if err = rows.Scan(&start, &size, &period, &retained); err != nil {
			fail(w, 500, "RETENTION_FAILED", "Не удалось прочитать когорты удержания")
			return
		}
		key := start.Format(time.RFC3339)
		if cohorts[key] == nil {
			cells := make([]cohortCell, periods)
			for i := range cells {
				cells[i] = cohortCell{Period: i, Label: fmt.Sprintf("%s%d", prefix, i)}
			}
			cohorts[key] = &cohort{Start: start, Size: size, Cells: cells}
			order = append(order, key)
		}
		if period != nil && retained != nil {
			cohorts[key].Cells[*period].Retained = *retained
			cohorts[key].Cells[*period].Rate = percentage(*retained, size)
		}
	}
	items := make([]map[string]any, 0, len(order))
	for _, key := range order {
		item := cohorts[key]
		items = append(items, map[string]any{"cohortStart": item.Start, "cohortSize": item.Size, "cells": item.Cells})
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{
		"cohortType": cohortType, "grain": grain, "periods": periods, "cohorts": items,
		"filters":     map[string]string{"branchId": branchID, "source": source, "campaignId": campaignID, "programId": programID},
		"limitations": []string{"non_participant_cohorts_require_anonymous_customer_identity", "industry_template_dimension_requires_program_template_attribution"},
	}})
}
