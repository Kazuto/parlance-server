package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// getLocaleFromRequest extracts locale preference from request headers
func getLocaleFromRequest[T any](req *connect.Request[T]) string {
	acceptLang := req.Header().Get("Accept-Language")

	if acceptLang != "" && len(acceptLang) >= 2 {
		return acceptLang[:2]
	}

	return "en"
}

// ListLocales returns a paginated list of locales
func (s *Server) ListLocales(
	ctx context.Context,
	req *connect.Request[pb.ListLocalesRequest],
) (*connect.Response[pb.ListLocalesResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)
	requestLocale := getLocaleFromRequest(req)

	var locales []models.Locale
	var total int64

	query := s.db.Model(&models.Locale{})

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count locales: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("code ASC").Find(&locales).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch locales: %w", err))
	}

	return connect.NewResponse(&pb.ListLocalesResponse{
		Locales:    converter.LocalesToProto(locales, requestLocale),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
