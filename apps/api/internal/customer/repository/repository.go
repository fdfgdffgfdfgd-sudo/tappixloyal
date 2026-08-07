package repository

import (
	"context"

	"github.com/tappix/platform/apps/api/internal/customer/entity"
)

type ListFilter struct {
	Search string
	Limit  int
	Offset int
}

type Repository interface {
	Create(context.Context, *entity.Customer) error
	GetByID(context.Context, string, string) (*entity.Customer, error)
	List(context.Context, string, ListFilter) ([]entity.Customer, int, error)
}
