package role

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteRole soft deletes a Role
func (s *Server) DeleteRole(
	ctx context.Context,
	req *connect.Request[pb.DeleteRoleRequest],
) (*connect.Response[pb.DeleteRoleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Role id is required"))
	}

	var role models.Role
	if err := s.db.First(&role, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Role not found"))
	}

	if err := s.db.Delete(&role).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete Role: %w", err))
	}

	return connect.NewResponse(&pb.DeleteRoleResponse{}), nil
}
