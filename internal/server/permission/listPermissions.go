package permission

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

// ListPermissions returns a paginated list of Permissions
func (s *Server) ListPermissions(
	ctx context.Context,
	req *connect.Request[pb.ListPermissionsRequest],
) (*connect.Response[pb.ListPermissionsResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var permissions []models.Permission
	var total int64

	query := s.db.Model(&models.Permission{})

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count Permissions: %w", err))
	}

	allowedSorts := map[string]bool{"name": true, "slug": true, "created_at": true, "updated_at": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("name ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("name LIKE ?", "%"+search+"%").Where("resource LIKE ?", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Find(&permissions).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch Permissions: %w", err))
	}

	permissionsProto := make([]*pb.Permission, len(permissions))
	for i, permission := range permissions {
		permissionsProto[i] = converter.PermissionToProto(&permission)
	}

	return connect.NewResponse(&pb.ListPermissionsResponse{
		Permissions: permissionsProto,
		Pagination:  converter.PaginationToProto(total, page, perPage),
	}), nil
}
