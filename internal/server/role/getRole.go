package role

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetRole returns a single Role by ID
func (s *Server) GetRole(
	ctx context.Context,
	req *connect.Request[pb.GetRoleRequest],
) (*connect.Response[pb.GetRoleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Role id is required"))
	}

	var role models.Role

	if err := s.db.First(&role, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Role not found"))
	}

	return connect.NewResponse(&pb.GetRoleResponse{
		Role: converter.RoleToProto(&role),
	}), nil
}
