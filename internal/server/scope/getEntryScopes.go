package scope

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetEntryScopes returns all scopes for an entry
func (s *Server) GetEntryScopes(
	ctx context.Context,
	req *connect.Request[pb.GetEntryScopesRequest],
) (*connect.Response[pb.GetEntryScopesResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	var entry models.Entry
	if err := s.db.Preload("Scopes").First(&entry, "id = ?", req.Msg.EntryId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	return connect.NewResponse(&pb.GetEntryScopesResponse{
		Scopes: converter.ScopesToProto(entry.Scopes),
	}), nil
}
