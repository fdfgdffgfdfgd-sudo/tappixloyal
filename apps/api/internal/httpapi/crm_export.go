package httpapi

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
)

func (a *api) exportCustomers(w http.ResponseWriter, r *http.Request) {
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	level := strings.TrimSpace(r.URL.Query().Get("level"))
	branch := strings.TrimSpace(r.URL.Query().Get("branch"))
	birthday := strings.TrimSpace(r.URL.Query().Get("birthday"))
	minPoints := clamp(parseInt(r.URL.Query().Get("minPoints"), 0), 0, 100000000)
	rows, err := a.db.Query(r.Context(), `SELECT c.first_name,c.last_name,c.phone,coalesce(to_char(c.birthday,'YYYY-MM-DD'),''),c.total_visits,c.total_points,c.level,to_char(c.created_at,'YYYY-MM-DD HH24:MI') FROM customers c WHERE c.company_id=$1 AND c.deleted_at IS NULL AND (c.first_name ILIKE $2 OR c.last_name ILIKE $2 OR c.phone ILIKE $2) AND ($3='' OR c.level=$3) AND c.total_points >= $4 AND ($5='' OR EXISTS(SELECT 1 FROM visits v WHERE v.company_id=c.company_id AND v.customer_id=c.id AND v.branch_id=nullif($5,'')::uuid)) AND ($6='' OR ($6='today' AND extract(month from c.birthday)=extract(month from current_date) AND extract(day from c.birthday)=extract(day from current_date)) OR ($6='month' AND extract(month from c.birthday)=extract(month from current_date))) ORDER BY c.created_at DESC`, companyID(r), search, level, minPoints, branch, birthday)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR", "Не удалось подготовить экспорт")
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="tappix-customers.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"Имя", "Фамилия", "Телефон", "Дата рождения", "Посещения", "Бонусы", "Уровень", "Регистрация"})
	for rows.Next() {
		var first, last, phone, birthday, levelName, created string
		var visits, points int
		if rows.Scan(&first, &last, &phone, &birthday, &visits, &points, &levelName, &created) == nil {
			_ = writer.Write([]string{first, last, phone, birthday, strconv.Itoa(visits), strconv.Itoa(points), levelName, created})
		}
	}
	writer.Flush()
}
