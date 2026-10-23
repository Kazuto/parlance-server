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

// GetEntryScopes returns all scopes for an entry
func (s *Server) GetEntryScopes(
	ctx context.Context,
	req *connect.Request[pb.GetEntryScopesRequest],
) (*connect.Response[pb.GetEntryScopesResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	entry, err := common.FindByID[models.Entry](s.db.WithContext(ctx).Preload("Scopes"), req.Msg.EntryId, errors.ErrEntryNotFound, "entry not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetEntryScopesResponse{
		Scopes: converter.ScopesToProto(entry.Scopes),
	}), nil
}
