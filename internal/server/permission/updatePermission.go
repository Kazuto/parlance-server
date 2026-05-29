package permission

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdatePermission updates an existing Permission
func (s *Server) UpdatePermission(
	ctx context.Context,
	req *connect.Request[pb.UpdatePermissionRequest],
) (*connect.Response[pb.UpdatePermissionResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Permission id is required"))
	}

	var permission models.Permission
	if err := s.db.First(&permission, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Permission not found"))
	}

	if req.Msg.Name != "" {
		permission.Name = req.Msg.Name
	}

	if req.Msg.Resource != "" {
		permission.Resource = req.Msg.Resource
	}

	if req.Msg.Action != "" {
		permission.Action = req.Msg.Action
	}

	if req.Msg.Description != "" {
		permission.Description = req.Msg.Description
	}

	if err := s.db.Save(&permission).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update Permission: %w", err))
	}

	return connect.NewResponse(&pb.UpdatePermissionResponse{
		Permission: converter.PermissionToProto(&permission),
	}), nil
}
