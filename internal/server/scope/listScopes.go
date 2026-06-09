package scope

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

// ListScopes returns a paginated list of scopes
func (s *Server) ListScopes(
	ctx context.Context,
	req *connect.Request[pb.ListScopesRequest],
) (*connect.Response[pb.ListScopesResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var scopes []models.Scope
	var total int64

	query := s.db.Model(&models.Scope{})

	allowedSorts := map[string]bool{"name": true, "slug": true, "created_at": true, "updated_at": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("name ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("name LIKE ?", "%"+search+"%").Where("slug LIKE ?", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count scopes: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Find(&scopes).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch scopes: %w", err))
	}

	return connect.NewResponse(&pb.ListScopesResponse{
		Scopes:     converter.ScopesToProto(scopes),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
