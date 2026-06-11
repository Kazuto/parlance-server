package role

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreRole(
	ctx context.Context,
	req *connect.Request[pb.RestoreRoleRequest],
) (*connect.Response[pb.RestoreRoleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Role id is required"))
	}

	var role models.Role
	if err := s.db.Preload("Permissions").First(&role, "id = ?", req.Msg.Id).Unscoped().Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Role not found"))
	}

	role.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&role).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to restore Role: %w", err))
	}

	return connect.NewResponse(&pb.RestoreRoleResponse{
		Role: converter.RoleToProto(&role),
	}), nil
}
