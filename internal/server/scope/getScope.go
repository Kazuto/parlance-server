package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetScope returns a single scope by ID
func (s *Server) GetScope(
	ctx context.Context,
	req *connect.Request[pb.GetScopeRequest],
) (*connect.Response[pb.GetScopeResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope id is required")
	}

	scope, err := common.FindByID[models.Scope](s.db.WithContext(ctx), req.Msg.Id, errors.ErrScopeNotFound, "scope not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}
