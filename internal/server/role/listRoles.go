package role

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

// ListRoles returns a paginated list of Roles
func (s *Server) ListRoles(
	ctx context.Context,
	req *connect.Request[pb.ListRolesRequest],
) (*connect.Response[pb.ListRolesResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadRole); err != nil {
		return nil, err
	}

	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var roles []models.Role
	var total int64

	query := s.db.Model(&models.Role{})

	allowedSorts := map[string]bool{"name": true, "created_at": true, "updated_at": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("name ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("name LIKE ?", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to count Roles")
	}

	offset := (page - 1) * perPage
	if err := query.Preload("Permissions").Offset(int(offset)).Limit(int(perPage)).Find(&roles).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to fetch Roles")
	}

	rolesProto := make([]*pb.Role, len(roles))
	for i, role := range roles {
		rolesProto[i] = converter.RoleToProto(&role)
	}

	return connect.NewResponse(&pb.ListRolesResponse{
		Roles:      rolesProto,
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
