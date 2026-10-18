package role

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateRole creates a new Role
func (s *Server) CreateRole(
	ctx context.Context,
	req *connect.Request[pb.CreateRoleRequest],
) (*connect.Response[pb.CreateRoleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.CreateRole); err != nil {
		return nil, err
	}

	if req.Msg.Name == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	if req.Msg.Description == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "action is required")
	}

	// Check if Role already exists
	var existingRole models.Role
	if err := s.db.Where("name = ?", req.Msg.Name).First(&existingRole).Error; err == nil {
		return nil, errors.AlreadyExists(errors.ErrRoleAlreadyExists, "role with this email already exists")
	}

	role := &models.Role{
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
	}

	if ids := req.Msg.GetPermissions().GetIds(); len(ids) > 0 {
		permissions, err := s.GetPermissions(ids)
		if err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to get permissions")
		}

		role.Permissions = permissions
	}

	if err := s.db.Create(role).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create Role")
	}

	return connect.NewResponse(&pb.CreateRoleResponse{
		Role: converter.RoleToProto(role),
	}), nil
}
