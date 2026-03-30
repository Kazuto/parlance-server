package scope

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetScope returns a single scope by ID
func (s *Server) GetScope(
	ctx context.Context,
	req *connect.Request[pb.GetScopeRequest],
) (*connect.Response[pb.GetScopeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	return connect.NewResponse(&pb.GetScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}
