package common

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"gorm.io/gorm"
)

type Filter struct {
	query        *gorm.DB
	filter       *pb.FilterRequest
	allowedSorts map[string]bool
	defaultSort  string
	searchFn     func(*gorm.DB, string) *gorm.DB
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

func (f *Filter) Apply() (*gorm.DB, error) {
	if f.filter == nil {
		return f.query, nil
	}

	if f.filter.Search != "" && f.searchFn != nil {
		f.query = f.searchFn(f.query, f.filter.Search)
	}

	query, err := f.applySort()
	if err != nil {
		return nil, err
	}

	if f.filter.IncludeDeleted {
		query = query.Unscoped()
	}

	return query, nil
}

func (f *Filter) applySort() (*gorm.DB, error) {
	if f.filter == nil {
		return f.query, nil
	}

	allowedOrders := map[string]bool{"asc": true, "desc": true}

	if f.filter.Sort != "" {
		sort := strings.ToLower(f.filter.Sort)
		order := strings.ToLower(f.filter.Order)

		if !f.allowedSorts[sort] {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid sort field: %s", sort))
		}

		if order != "" && !allowedOrders[order] {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid order direction: %s", order))
		}

		if order == "" {
			order = "asc"
		}

		return f.query.Order(sort + " " + order), nil
	}

	return f.query.Order(f.defaultSort), nil
}
