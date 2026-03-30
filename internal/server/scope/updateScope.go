package scope

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateScope updates an existing scope
func (s *Server) UpdateScope(
	ctx context.Context,
	req *connect.Request[pb.UpdateScopeRequest],
) (*connect.Response[pb.UpdateScopeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	if req.Msg.Name != "" {
		scope.Name = req.Msg.Name
	}

	if req.Msg.Slug != "" {
		scope.Slug = req.Msg.Slug
	}

	if req.Msg.Description != "" {
		scope.Description = req.Msg.Description
	}

	if req.Msg.Color != "" {
		scope.Color = req.Msg.Color
	}

	if err := s.db.Save(&scope).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update scope: %w", err))
	}

	return connect.NewResponse(&pb.UpdateScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}
