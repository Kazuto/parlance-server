package scope

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// AddScopeToEntry adds a scope to an entry
func (s *Server) AddScopeToEntry(
	ctx context.Context,
	req *connect.Request[pb.AddScopeToEntryRequest],
) (*connect.Response[pb.AddScopeToEntryResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	if req.Msg.ScopeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var entry models.Entry
	if err := s.db.First(&entry, "id = ?", req.Msg.EntryId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.ScopeId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	if err := s.db.Model(&entry).Association("Scopes").Append(&scope); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add scope to entry: %w", err))
	}

	return connect.NewResponse(&pb.AddScopeToEntryResponse{}), nil
}
