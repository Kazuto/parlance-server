package role

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteRole soft deletes a Role
func (s *Server) DeleteRole(
	ctx context.Context,
	req *connect.Request[pb.DeleteRoleRequest],
) (*connect.Response[pb.DeleteRoleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteRole); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "role id is required")
	}

	var role models.Role
	if err := s.db.First(&role, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrRoleNotFound, "role not found")
	}

	if err := s.db.Delete(&role).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete Role")
	}

	return connect.NewResponse(&pb.DeleteRoleResponse{}), nil
}
