package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

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
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope id is required")
	}
	if req.Msg.Name != nil && req.Msg.GetName() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope name is required")
	}
	if req.Msg.Slug != nil && req.Msg.GetSlug() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope slug is required")
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrScopeNotFound, "scope not found")
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
		return nil, errors.Internal(errors.ErrInternal, "failed to update scope")
	}

	return connect.NewResponse(&pb.UpdateScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}
