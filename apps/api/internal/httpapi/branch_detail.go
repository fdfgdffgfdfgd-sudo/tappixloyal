package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *api) branchDetail(w http.ResponseWriter, r *http.Request) {
	tenant, id := companyID(r), r.PathValue("id")
	var name, address, phone string
	var active bool
	if err := a.db.QueryRow(r.Context(), `SELECT name,address,coalesce(phone,''),is_active FROM branches WHERE company_id=$1 AND id=$2 AND deleted_at IS NULL`, tenant, id).Scan(&name, &address, &phone, &active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "BRANCH_NOT_FOUND", "Филиал не найден")
		} else {
			fail(w, 500, "DATABASE_ERROR", "Не удалось загрузить филиал")
		}
		return
	}
	var visits, customers, points int
	_ = a.db.QueryRow(r.Context(), `SELECT count(*),count(distinct customer_id),coalesce(sum(points_added),0) FROM visits WHERE company_id=$1 AND branch_id=$2 AND created_at >= now()-interval '30 days'`, tenant, id).Scan(&visits, &customers, &points)
	employees := []map[string]any{}
	rows, err := a.db.Query(r.Context(), `SELECT id,first_name,last_name,email,status,last_login_at FROM users WHERE company_id=$1 AND branch_id=$2 AND role='employee' AND deleted_at IS NULL ORDER BY first_name,last_name`, tenant, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var employeeID, first, last, email, status string
			var lastLogin *time.Time
			if rows.Scan(&employeeID, &first, &last, &email, &status, &lastLogin) == nil {
				employees = append(employees, map[string]any{"id": employeeID, "firstName": first, "lastName": last, "email": email, "status": status, "lastLoginAt": lastLogin})
			}
		}
	}
	devices := []map[string]any{}
	deviceRows, err := a.db.Query(r.Context(), `SELECT id,kind,name,token,destination,is_active,scans_count,last_scanned_at FROM devices WHERE company_id=$1 AND branch_id=$2 ORDER BY created_at DESC`, tenant, id)
	if err == nil {
		defer deviceRows.Close()
		for deviceRows.Next() {
			var deviceID, kind, deviceName, token, destination string
			var enabled bool
			var scans int64
			var lastScan *time.Time
			if deviceRows.Scan(&deviceID, &kind, &deviceName, &token, &destination, &enabled, &scans, &lastScan) == nil {
				devices = append(devices, map[string]any{"id": deviceID, "kind": kind, "name": deviceName, "token": token, "destination": destination, "active": enabled, "scans": scans, "lastScannedAt": lastScan})
			}
		}
	}
	write(w, 200, envelope{Success: true, Data: map[string]any{"id": id, "name": name, "address": address, "phone": phone, "active": active, "stats": map[string]int{"visits30Days": visits, "uniqueCustomers30Days": customers, "points30Days": points}, "employees": employees, "devices": devices}})
}
