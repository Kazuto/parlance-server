package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListLocales returns a paginated list of locales
func (s *Server) ListLocales(
	ctx context.Context,
	req *connect.Request[pb.ListLocalesRequest],
) (*connect.Response[pb.ListLocalesResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)
	requestLocale := common.GetLocaleFromRequest(req)

	var locales []models.Locale
	var total int64

	query := s.db.Model(&models.Locale{})

	allowedSorts := map[string]bool{"code": true, "name": true, "created_at": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("code ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("code LIKE ?", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count locales: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Find(&locales).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch locales: %w", err))
	}

	return connect.NewResponse(&pb.ListLocalesResponse{
		Locales:    converter.LocalesToProto(locales, requestLocale),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
