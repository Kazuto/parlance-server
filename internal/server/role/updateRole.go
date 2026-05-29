package role

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateRole updates an existing Role
func (s *Server) UpdateRole(
	ctx context.Context,
	req *connect.Request[pb.UpdateRoleRequest],
) (*connect.Response[pb.UpdateRoleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Role id is required"))
	}

	var role models.Role
	if err := s.db.Preload("Permissions").First(&role, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Role not found"))
	}

	if req.Msg.Name != "" {
		role.Name = req.Msg.Name
	}

	if req.Msg.Description != "" {
		role.Description = req.Msg.Description
	}

	if err := s.db.Save(&role).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update Role: %w", err))
	}

	return connect.NewResponse(&pb.UpdateRoleResponse{
		Role: converter.RoleToProto(&role),
	}), nil
}
