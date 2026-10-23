package locale

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListLocales returns a paginated list of locales
func (s *Server) ListLocales(
	ctx context.Context,
	req *connect.Request[pb.ListLocalesRequest],
) (*connect.Response[pb.ListLocalesResponse], error) {
	query, err := common.NewFilter(s.db.WithContext(ctx).Model(&models.Locale{}), req.Msg.Filter).
		IncludeDeletedRequires(ctx, s.db, permissions.PermanentDeleteLocale).
		AllowedSorts(map[string]bool{"code": true, "name": true, "created_at": true}).
		DefaultSort("code ASC").
		Search(common.SearchColumns("code")).
		Apply()
	if err != nil {
		return nil, err
	}

	var locales []models.Locale

	pagination, err := common.Paginate(query, req.Msg.Pagination, &locales)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListLocalesResponse{
		Locales:    converter.LocalesToProto(locales, common.GetLocaleFromRequest(req)),
		Pagination: pagination,
	}), nil
}
