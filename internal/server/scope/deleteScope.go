package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteScope soft deletes a scope
func (s *Server) DeleteScope(
	ctx context.Context,
	req *connect.Request[pb.DeleteScopeRequest],
) (*connect.Response[pb.DeleteScopeResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope id is required")
	}

	scope, err := common.FindByID[models.Scope](s.db.WithContext(ctx), req.Msg.Id, errors.ErrScopeNotFound, "scope not found")
	if err != nil {
		return nil, err
	}

	if err := common.DeleteRow(ctx, s.db.DB, &scope); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete scope")
	}

	return connect.NewResponse(&pb.DeleteScopeResponse{}), nil
}
