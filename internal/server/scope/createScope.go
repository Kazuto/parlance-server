package scope

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateScope creates a new scope
func (s *Server) CreateScope(
	ctx context.Context,
	req *connect.Request[pb.CreateScopeRequest],
) (*connect.Response[pb.CreateScopeResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope name is required"))
	}

	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope slug is required"))
	}

	scope := &models.Scope{
		Name:        req.Msg.Name,
		Slug:        req.Msg.Slug,
		Description: req.Msg.Description,
		Color:       req.Msg.Color,
	}

	if err := s.db.Create(scope).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create scope: %w", err))
	}

	return connect.NewResponse(&pb.CreateScopeResponse{
		Scope: converter.ScopeToProto(scope),
	}), nil
}
