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

// GetScope returns a single scope by ID
func (s *Server) GetScope(
	ctx context.Context,
	req *connect.Request[pb.GetScopeRequest],
) (*connect.Response[pb.GetScopeResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadScope); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope id is required")
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrScopeNotFound, "scope not found")
	}

	return connect.NewResponse(&pb.GetScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}
