package permission

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetPermission returns a single Permission by ID
func (s *Server) GetPermission(
	ctx context.Context,
	req *connect.Request[pb.GetPermissionRequest],
) (*connect.Response[pb.GetPermissionResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Permission id is required"))
	}

	var permission models.Permission

	if err := s.db.First(&permission, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Permission not found"))
	}

	return connect.NewResponse(&pb.GetPermissionResponse{
		Permission: converter.PermissionToProto(&permission),
	}), nil
}
