package role

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"gorm.io/gorm"
)

// UpdateRole updates an existing Role
func (s *Server) UpdateRole(
	ctx context.Context,
	req *connect.Request[pb.UpdateRoleRequest],
) (*connect.Response[pb.UpdateRoleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateRole); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Role id is required"))
	}
	if req.Msg.Name != nil && req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	var role models.Role
	if err := s.db.Preload("Permissions").First(&role, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Role not found"))
	}

	if req.Msg.Name != nil {
		role.Name = req.Msg.GetName()
	}

	if req.Msg.Description != nil {
		role.Description = req.Msg.GetDescription()
	}

	var permissions []models.Permission
	if req.Msg.Permissions != nil {
		var err error
		permissions, err = s.GetPermissions(req.Msg.Permissions.GetIds())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to get permissions: %w", err))
		}
	}

	// Save only adds many2many links, so a sent selection replaces them explicitly.
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Permissions").Save(&role).Error; err != nil {
			return err
		}
		if req.Msg.Permissions != nil {
			return tx.Model(&role).Association("Permissions").Replace(permissions)
		}
		return nil
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update Role: %w", err))
	}

	return connect.NewResponse(&pb.UpdateRoleResponse{
		Role: converter.RoleToProto(&role),
	}), nil
}
