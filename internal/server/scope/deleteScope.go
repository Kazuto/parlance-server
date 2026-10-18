package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteScope soft deletes a scope
func (s *Server) DeleteScope(
	ctx context.Context,
	req *connect.Request[pb.DeleteScopeRequest],
) (*connect.Response[pb.DeleteScopeResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteScope); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope id is required")
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrScopeNotFound, "scope not found")
	}

	if err := s.db.Delete(&scope).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete scope")
	}

	return connect.NewResponse(&pb.DeleteScopeResponse{}), nil
}
