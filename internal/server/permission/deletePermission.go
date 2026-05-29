package permission

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeletePermission soft deletes a Permission
func (s *Server) DeletePermission(
	ctx context.Context,
	req *connect.Request[pb.DeletePermissionRequest],
) (*connect.Response[pb.DeletePermissionResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Permission id is required"))
	}

	var permission models.Permission
	if err := s.db.First(&permission, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Permission not found"))
	}

	if err := s.db.Delete(&permission).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete Permission: %w", err))
	}

	return connect.NewResponse(&pb.DeletePermissionResponse{}), nil
}
