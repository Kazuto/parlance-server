package user

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// AssignRole assigns a role to a user
func (s *Server) AssignRole(
	ctx context.Context,
	req *connect.Request[pb.AssignRoleRequest],
) (*connect.Response[pb.AssignRoleResponse], error) {
	if req.Msg.UserId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user id is required"))
	}

	if req.Msg.RoleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("role id is required"))
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", req.Msg.UserId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	var role models.Role
	if err := s.db.First(&role, "id = ?", req.Msg.RoleId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("role not found"))
	}

	if err := s.db.Model(&user).Association("Roles").Append(&role); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to assign role: %w", err))
	}

	// Load updated roles for response
	s.db.Preload("Roles.Permissions").First(&user, "id = ?", user.ID)

	return connect.NewResponse(&pb.AssignRoleResponse{
		User: converter.UserToProto(&user),
	}), nil
}
