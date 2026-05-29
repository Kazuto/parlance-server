package permission

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreatePermission creates a new Permission
func (s *Server) CreatePermission(
	ctx context.Context,
	req *connect.Request[pb.CreatePermissionRequest],
) (*connect.Response[pb.CreatePermissionResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	if req.Msg.Resource == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("resource is required"))
	}

	if req.Msg.Action == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("action is required"))
	}

	// Check if Permission already exists
	var existingPermission models.Permission
	if err := s.db.Where("resource = ?", req.Msg.Resource).Where("action = ?", req.Msg.Action).First(&existingPermission).Error; err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("Permission with this email already exists"))
	}

	permission := &models.Permission{
		Name:        req.Msg.Name,
		Resource:    req.Msg.Resource,
		Action:      req.Msg.Action,
		Description: req.Msg.Description,
	}

	if err := s.db.Create(permission).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create Permission: %w", err))
	}

	return connect.NewResponse(&pb.CreatePermissionResponse{
		Permission: converter.PermissionToProto(permission),
	}), nil
}
