package common

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"gorm.io/gorm"
)

type Filter struct {
	query         *gorm.DB
	filter        *pb.FilterRequest
	allowedSorts  map[string]bool
	defaultSort   string
	searchFn      func(*gorm.DB, string) *gorm.DB
	canSeeDeleted func() bool
}

func NewFilter(query *gorm.DB, filter *pb.FilterRequest) *Filter {
	return &Filter{query: query, filter: filter}
}

func (f *Filter) AllowedSorts(sorts map[string]bool) *Filter {
	f.allowedSorts = sorts

	return f
}

func (f *Filter) DefaultSort(sort string) *Filter {
	f.defaultSort = sort

	return f
}

func (f *Filter) Search(fn func(*gorm.DB, string) *gorm.DB) *Filter {
	f.searchFn = fn

	return f
}

// IncludeDeletedRequires names the permission needed to list soft-deleted rows. Without it,
// or for callers lacking it, include_deleted is ignored.
func (f *Filter) IncludeDeletedRequires(ctx context.Context, db *database.DB, permission string) *Filter {
	f.canSeeDeleted = func() bool { return middleware.HasPermission(ctx, db, permission) }

	return f
}

func (f *Filter) Apply() (*gorm.DB, error) {
	if f.filter != nil && f.filter.Search != "" && f.searchFn != nil {
		f.query = f.searchFn(f.query, f.filter.Search)
	}

	query, err := f.applySort()
	if err != nil {
		return nil, err
	}

	if f.filter != nil && f.filter.IncludeDeleted && f.canSeeDeleted != nil && f.canSeeDeleted() {
		query = query.Unscoped()
	}

	return query, nil
}

func (f *Filter) applySort() (*gorm.DB, error) {
	if f.filter == nil || f.filter.Sort == "" {
		if f.defaultSort == "" {
			return f.query, nil
		}

		return f.query.Order(f.defaultSort), nil
	}

	allowedOrders := map[string]bool{"asc": true, "desc": true}

	sort := strings.ToLower(f.filter.Sort)
	order := strings.ToLower(f.filter.Order)

	if !f.allowedSorts[sort] {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, fmt.Sprintf("invalid sort field: %s", sort))
	}

	if order != "" && !allowedOrders[order] {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, fmt.Sprintf("invalid order direction: %s", order))
	}

	if order == "" {
		order = "asc"
	}

	return f.query.Order(sort + " " + order), nil
}
