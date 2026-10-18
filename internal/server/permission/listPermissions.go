package permission

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListPermissions returns a paginated list of Permissions
func (s *Server) ListPermissions(
	ctx context.Context,
	req *connect.Request[pb.ListPermissionsRequest],
) (*connect.Response[pb.ListPermissionsResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadPermission); err != nil {
		return nil, err
	}

	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var permissions []models.Permission
	var total int64

	query := s.db.Model(&models.Permission{})

	allowedSorts := map[string]bool{"name": true, "resource": true, "action": true, "created_at": true, "updated_at": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("name ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("name ILIKE ? OR resource ILIKE ?", "%"+search+"%", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	// Count after filtering so the total and page count match the search.
	if err := query.Count(&total).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to count Permissions")
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Find(&permissions).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to fetch Permissions")
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
