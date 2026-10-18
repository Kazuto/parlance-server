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

// GetRole returns a single Role by ID
func (s *Server) GetRole(
	ctx context.Context,
	req *connect.Request[pb.GetRoleRequest],
) (*connect.Response[pb.GetRoleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadRole); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "role id is required")
	}

	var role models.Role
	if err := s.db.Preload("Permissions").First(&role, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrRoleNotFound, "role not found")
	}

	return connect.NewResponse(&pb.GetRoleResponse{
		Role: converter.RoleToProto(&role),
	}), nil
}
