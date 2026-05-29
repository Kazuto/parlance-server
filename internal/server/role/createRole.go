package role

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateRole creates a new Role
func (s *Server) CreateRole(
	ctx context.Context,
	req *connect.Request[pb.CreateRoleRequest],
) (*connect.Response[pb.CreateRoleResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	if req.Msg.Description == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("action is required"))
	}

	// Check if Role already exists
	var existingRole models.Role
	if err := s.db.Where("name = ?", req.Msg.Name).First(&existingRole).Error; err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("Role with this email already exists"))
	}

	role := &models.Role{
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
	}

	if err := s.db.Create(role).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create Role: %w", err))
	}

	return connect.NewResponse(&pb.CreateRoleResponse{
		Role: converter.RoleToProto(role),
	}), nil
}
