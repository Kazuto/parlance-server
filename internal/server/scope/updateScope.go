package scope

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateScope updates an existing scope
func (s *Server) UpdateScope(
	ctx context.Context,
	req *connect.Request[pb.UpdateScopeRequest],
) (*connect.Response[pb.UpdateScopeResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateScope); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}
	if req.Msg.Name != nil && req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope name is required"))
	}
	if req.Msg.Slug != nil && req.Msg.GetSlug() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope slug is required"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	if req.Msg.Name != nil {
		scope.Name = req.Msg.GetName()
	}

	if req.Msg.Slug != nil {
		scope.Slug = req.Msg.GetSlug()
	}

	if req.Msg.Description != nil {
		scope.Description = req.Msg.GetDescription()
	}

	if req.Msg.Color != nil {
		scope.Color = req.Msg.GetColor()
	}

	if err := s.db.Save(&scope).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update scope: %w", err))
	}

	return connect.NewResponse(&pb.UpdateScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}
