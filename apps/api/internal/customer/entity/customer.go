package entity

import "time"

type Customer struct {
	ID          string
	CompanyID   string
	FirstName   string
	LastName    string
	Phone       string
	Birthday    *time.Time
	TotalPoints int
	TotalVisits int
	Level       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
